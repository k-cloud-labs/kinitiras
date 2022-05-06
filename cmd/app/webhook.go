package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/util/informermanager"
	"github.com/k-cloud-labs/pkg/util/overridemanager"
	"github.com/k-cloud-labs/pkg/util/validatemanager"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/k-cloud-labs/kinitiras/cmd/app/options"
	"github.com/k-cloud-labs/kinitiras/pkg/controller/cert"
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

	overrideManager, err := setupOverridePolicyManager(informerManager)
	if err != nil {
		klog.ErrorS(err, "failed to setup override policy manager.")
		return err
	}

	validateManager, err := setupValidatePolicyManager(informerManager)
	if err != nil {
		klog.ErrorS(err, "failed to setup validate policy manager.")
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
		hookServer.Register("/mutate", &webhook.Admission{Handler: pkgwebhook.NewMutatingAdmissionHandler(overrideManager)})
		hookServer.Register("/validate", &webhook.Admission{Handler: pkgwebhook.NewValidatingAdmissionHandler(validateManager)})
		hookServer.Register("/readyz", &healthz.Handler{})
	}()

	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.ErrorS(err, "webhook server exits unexpectedly.")
		return err
	}

	// never reach here
	return nil
}

func setupOverridePolicyManager(informerManager informermanager.SingleClusterInformerManager) (overridemanager.OverrideManager, error) {
	opGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "overridepolicy",
	}
	copGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "clusteroverridepolicy",
	}
	opInformer := informerManager.Informer(opGVR)
	copInformer := informerManager.Informer(copGVR)

	informerManager.Start()

	if cache := informerManager.WaitForCacheSync(); !cache[opGVR] || !cache[copGVR] {
		return nil, errors.New("failed to sync override policy.")
	}

	return overridemanager.NewOverrideManager(v1alpha1.NewClusterOverridePolicyLister(copInformer.GetIndexer()),
		v1alpha1.NewOverridePolicyLister(opInformer.GetIndexer())), nil
}

func setupValidatePolicyManager(informerManager informermanager.SingleClusterInformerManager) (validatemanager.ValidateManager, error) {
	cvpGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.SchemeGroupVersion.Group,
		Version:  policyv1alpha1.SchemeGroupVersion.Version,
		Resource: "clustervalidatepolicy",
	}
	cvpInformer := informerManager.Informer(cvpGVR)

	informerManager.Start()

	if cache := informerManager.WaitForCacheSync(); !cache[cvpGVR] {
		return nil, errors.New("failed to sync validate policy.")
	}

	return validatemanager.NewValidateManager(v1alpha1.NewClusterValidatePolicyLister(cvpInformer.GetIndexer())), nil
}
