package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	"github.com/k-cloud-labs/pkg/utils/informermanager"
	"github.com/k-cloud-labs/pkg/utils/interrupter"
	"github.com/k-cloud-labs/pkg/utils/metrics"
	"github.com/k-cloud-labs/pkg/utils/overridemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager/templates"
	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
	"github.com/k-cloud-labs/pkg/utils/validatemanager"

	"github.com/k-cloud-labs/kinitiras/cmd/app/options"
	"github.com/k-cloud-labs/kinitiras/pkg/controller/cert"
	"github.com/k-cloud-labs/kinitiras/pkg/lister"
	"github.com/k-cloud-labs/kinitiras/pkg/util/gclient"
	"github.com/k-cloud-labs/kinitiras/pkg/version"
	"github.com/k-cloud-labs/kinitiras/pkg/version/sharedcommand"
	pkgwebhook "github.com/k-cloud-labs/kinitiras/pkg/webhook"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "kinitiras-webhook",
		Long: `Start a mutating webhook server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.PrintFlags(cmd.Flags())

			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := Run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "kinitiras-webhook"))
	opts.AddFlags(cmd.Flags())

	return cmd
}

// Run runs the webhook server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.InfoS("kinitiras webhook starting.", "version", version.Get())
	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme: gclient.NewSchema(),
		WebhookServer: &webhook.Server{
			Host:          opts.BindAddress,
			Port:          opts.SecurePort,
			CertDir:       opts.CertDir,
			TLSMinVersion: opts.TLSMinVersion,
		},
		MetricsBindAddress: opts.MetricsBindAddress,
		LeaderElection:     false,
	})
	if err != nil {
		klog.ErrorS(err, "failed to build webhook server.")
		return err
	}

	if err := hookManager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.ErrorS(err, "failed to add health check endpoint.")
		return err
	}

	sm := &setupManager{
		opts: opts,
	}
	if err := sm.init(hookManager, ctx.Done()); err != nil {
		klog.ErrorS(err, "init setup manager failed")
		return err
	}

	if err := sm.waitForCacheSync(ctx); err != nil {
		klog.ErrorS(err, "wait for cache sync failed")
		return err
	}

	if err := sm.setupInterrupter(); err != nil {
		klog.ErrorS(err, "setup interrupter failed")
		return err
	}

	setupCh, err := cert.SetupCertRotator(hookManager, cert.Options{
		Namespace:      os.Getenv("NAMESPACE"),
		SecretName:     os.Getenv("SECRET"),
		CAOrganization: os.Getenv("CA_ORGANIZATION"),
		CAName:         os.Getenv("CA_NAME"),
		ServiceName:    os.Getenv("SERVICE_NAME"),
		CertDir:        opts.CertDir,
		Webhooks: []cert.WebhookInfo{
			{
				Name: os.Getenv("MUTATING_CONFIG"),
				Type: cert.Mutating,
			},
			{
				Name: os.Getenv("VALIDATING_CONFIG"),
				Type: cert.Validating,
			},
		},
	})
	if err != nil {
		klog.ErrorS(err, "failed to setup cert rotator controller.")
		return err
	}

	go func() {
		<-setupCh

		klog.InfoS("registering webhooks to the webhook server.")
		hookServer := hookManager.GetWebhookServer()
		hookServer.Register("/mutate", &webhook.Admission{Handler: pkgwebhook.NewMutatingAdmissionHandler(sm.overrideManager, sm.policyInterrupterManager)})
		hookServer.Register("/validate", &webhook.Admission{Handler: pkgwebhook.NewValidatingAdmissionHandler(sm.validateManager, sm.policyInterrupterManager)})
		hookServer.WebhookMux.Handle("/readyz", http.StripPrefix("/readyz", &healthz.Handler{}))
	}()

	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.ErrorS(err, "webhook server exits unexpectedly.")
		return err
	}

	// never reach here
	return nil
}

type setupManager struct {
	opts                     *options.Options
	hookManager              manager.Manager
	done                     <-chan struct{}
	client                   client.Client
	drLister                 dynamiclister.DynamicResourceLister
	opLister                 v1alpha1.OverridePolicyLister
	copLister                v1alpha1.ClusterOverridePolicyLister
	cvpLister                v1alpha1.ClusterValidatePolicyLister
	informerManager          informermanager.SingleClusterInformerManager
	overrideManager          overridemanager.OverrideManager
	validateManager          validatemanager.ValidateManager
	policyInterrupterManager interrupter.PolicyInterrupterManager
	tokenManager             tokenmanager.TokenManager
}

func (s *setupManager) init(hm manager.Manager, done <-chan struct{}) (err error) {
	s.hookManager = hm
	s.done = done
	s.informerManager = informermanager.NewSingleClusterInformerManager(dynamic.NewForConfigOrDie(hm.GetConfig()), 0, done)
	s.client = hm.GetClient()
	s.policyInterrupterManager = interrupter.NewPolicyInterrupterManager()
	s.tokenManager = tokenmanager.NewTokenManager()

	s.drLister, err = dynamiclister.NewDynamicResourceLister(hm.GetConfig(), done)
	if err != nil {
		klog.ErrorS(err, "failed to init dynamic client.")
		return err
	}

	return nil
}

func (s *setupManager) waitForCacheSync(ctx context.Context) error {
	eg, _ := errgroup.WithContext(ctx)
	eg.Go(func() error {
		// pre cached resources
		err := s.drLister.RegisterNewResource(true, s.opts.PreCacheResourcesToGVKList()...)
		if err != nil {
			klog.ErrorS(err, "failed to register resource to lister")
		}
		return err
	})
	eg.Go(func() error {
		if err := s.setupOverridePolicyManager(); err != nil {
			klog.ErrorS(err, "failed to setup override policy manager.")
			return err
		}
		return nil
	})

	eg.Go(func() error {

		if err := s.setupValidatePolicyManager(); err != nil {
			klog.ErrorS(err, "failed to setup validate policy manager.")
			return err
		}

		return nil
	})

	return eg.Wait()
}

func (s *setupManager) setupInterrupter() error {
	otm, err := templatemanager.NewOverrideTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.OverrideTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		klog.ErrorS(err, "failed to setup mutating template manager.")
		return err
	}

	vtm, err := templatemanager.NewValidateTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.ValidateTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		klog.ErrorS(err, "failed to setup validate template manager.")
		return err
	}

	// base
	baseInterrupter := interrupter.NewBaseInterrupter(otm, vtm, templatemanager.NewCueManager())

	// op
	overridePolicyInterrupter := interrupter.NewOverridePolicyInterrupter(baseInterrupter, s.tokenManager, s.client, s.opLister)
	s.policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "OverridePolicy",
	}, overridePolicyInterrupter)
	// cop
	s.policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ClusterOverridePolicy",
	}, interrupter.NewClusterOverridePolicyInterrupter(overridePolicyInterrupter, s.copLister))
	// cvp
	s.policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ClusterValidatePolicy",
	}, interrupter.NewClusterValidatePolicyInterrupter(baseInterrupter, s.tokenManager, s.client, s.cvpLister))

	return s.policyInterrupterManager.OnStartUp()
}

func (s *setupManager) setupOverridePolicyManager() (err error) {
	opGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "overridepolicies",
	}
	copGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "clusteroverridepolicies",
	}
	opInformer := s.informerManager.Informer(opGVR)
	copInformer := s.informerManager.Informer(copGVR)

	opInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			metrics.IncrPolicy("OverridePolicy")
		},
		DeleteFunc: func(obj interface{}) {
			metrics.DecPolicy("OverridePolicy")
		},
	})

	copInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			metrics.IncrPolicy("ClusterOverridePolicy")
		},
		DeleteFunc: func(obj interface{}) {
			metrics.DecPolicy("ClusterOverridePolicy")
		},
	})

	s.informerManager.Start()
	if result := s.informerManager.WaitForCacheSync(); !result[opGVR] || !result[copGVR] {
		return errors.New("failed to sync override policy")
	}

	s.opLister = lister.NewUnstructuredOverridePolicyLister(opInformer.GetIndexer())
	s.copLister = lister.NewUnstructuredClusterOverridePolicyLister(copInformer.GetIndexer())
	s.overrideManager = overridemanager.NewOverrideManager(s.drLister, s.copLister, s.opLister)
	return nil
}

func (s *setupManager) setupValidatePolicyManager() (err error) {
	cvpGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "clustervalidatepolicies",
	}
	cvpInformer := s.informerManager.Informer(cvpGVR)

	cvpInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			metrics.IncrPolicy("ClusterValidatePolicy")
		},
		DeleteFunc: func(obj interface{}) {
			metrics.DecPolicy("ClusterValidatePolicy")
		},
	})

	s.informerManager.Start()
	if result := s.informerManager.WaitForCacheSync(); !result[cvpGVR] {
		return errors.New("failed to sync validate policy")
	}

	s.cvpLister = lister.NewUnstructuredClusterValidatePolicyLister(cvpInformer.GetIndexer())
	s.validateManager = validatemanager.NewValidateManager(s.drLister, s.cvpLister)
	return nil
}
