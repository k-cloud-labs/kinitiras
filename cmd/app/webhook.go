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
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	"github.com/k-cloud-labs/pkg/utils/informermanager"
	"github.com/k-cloud-labs/pkg/utils/interrupter"
	"github.com/k-cloud-labs/pkg/utils/overridemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager/templates"
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

	informerManager := informermanager.NewSingleClusterInformerManager(dynamic.NewForConfigOrDie(hookManager.GetConfig()), 0, ctx.Done())

	resourceLister, err := dynamiclister.NewDynamicResourceLister(hookManager.GetConfig(), ctx.Done())
	if err != nil {
		klog.ErrorS(err, "failed to init dynamic client.")
		return err
	}

	eg, _ := errgroup.WithContext(ctx)
	var (
		overrideManager overridemanager.OverrideManager
		validateManager validatemanager.ValidateManager
	)

	eg.Go(func() error {
		err := resourceLister.RegisterNewResource(true,
			// pre cached resources
			schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}, schema.GroupVersionKind{
				Version: "v1",
				Kind:    "Pod",
			}, schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "ReplicaSet",
			})
		if err != nil {
			klog.ErrorS(err, "failed to register resource to lister")
		}
		return err
	})
	eg.Go(func() error {
		temp, err := setupOverridePolicyManager(resourceLister, informerManager)
		if err != nil {
			klog.ErrorS(err, "failed to setup override policy manager.")
			return err
		}

		overrideManager = temp
		return nil
	})

	eg.Go(func() error {
		temp, err := setupValidatePolicyManager(resourceLister, informerManager)
		if err != nil {
			klog.ErrorS(err, "failed to setup validate policy manager.")
			return err
		}

		validateManager = temp
		return nil
	})

	if err = eg.Wait(); err != nil {
		return err
	}

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

	policyInterrupter := interrupter.NewPolicyInterrupter(otm, vtm, templatemanager.NewCueManager())

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
		hookServer.Register("/mutate", &webhook.Admission{Handler: pkgwebhook.NewMutatingAdmissionHandler(overrideManager, policyInterrupter)})
		hookServer.Register("/validate", &webhook.Admission{Handler: pkgwebhook.NewValidatingAdmissionHandler(validateManager, policyInterrupter)})
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

func setupOverridePolicyManager(dc dynamiclister.DynamicResourceLister, informerManager informermanager.SingleClusterInformerManager) (overridemanager.OverrideManager, error) {
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
	opInformer := informerManager.Informer(opGVR)
	copInformer := informerManager.Informer(copGVR)

	informerManager.Start()

	if cache := informerManager.WaitForCacheSync(); !cache[opGVR] || !cache[copGVR] {
		return nil, errors.New("failed to sync override policy")
	}

	return overridemanager.NewOverrideManager(dc, lister.NewUnstructuredClusterOverridePolicyLister(copInformer.GetIndexer()),
		lister.NewUnstructuredOverridePolicyLister(opInformer.GetIndexer())), nil
}

func setupValidatePolicyManager(dc dynamiclister.DynamicResourceLister, informerManager informermanager.SingleClusterInformerManager) (validatemanager.ValidateManager, error) {
	cvpGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "clustervalidatepolicies",
	}
	cvpInformer := informerManager.Informer(cvpGVR)

	informerManager.Start()

	if cache := informerManager.WaitForCacheSync(); !cache[cvpGVR] {
		return nil, errors.New("failed to sync validate policy.")
	}

	return validatemanager.NewValidateManager(dc, lister.NewUnstructuredClusterValidatePolicyLister(cvpInformer.GetIndexer())), nil
}
