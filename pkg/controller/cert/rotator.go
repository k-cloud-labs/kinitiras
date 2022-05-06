package cert

import (
	"fmt"

	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	secretName     = "kinitiras-webhook-cert"
	namespace      = "kinitiras-system"
	caName         = "kinitiras-ca"
	caOrganization = "kinitiras"
	serviceName    = "kinitiras-webhook"
	certDir        = "/tmp/k8s-webhook-server/serving-certs"
)

type Options struct {
	Namespace      string
	SecretName     string
	CertDir        string
	CAName         string
	ServiceName    string
	CAOrganization string
	Webhooks       []WebhookInfo
}

func (option *Options) Default() {
	if option.Namespace == "" {
		option.Namespace = namespace
	}

	if option.SecretName == "" {
		option.SecretName = secretName
	}

	if option.CertDir == "" {
		option.CertDir = certDir
	}

	if option.CAOrganization == "" {
		option.CAOrganization = caOrganization
	}

	if option.CAName == "" {
		option.CAName = caName
	}

	if option.ServiceName == "" {
		option.ServiceName = serviceName
	}

	for i := range option.Webhooks {
		if option.Webhooks[i].Name == "" {
			option.Webhooks[i].Name = serviceName
		}
	}
}

type WebhookInfo struct {
	Name string
	Type WebhookType
}

//WebhookType is the type of webhook, either validating/mutating webhook, a CRD conversion webhook, or an extension API server
type WebhookType int

const (
	//ValidatingWebhook indicates the webhook is a ValidatingWebhook
	Validating WebhookType = iota
	//MutingWebhook indicates the webhook is a MutatingWebhook
	Mutating
	//CRDConversionWebhook indicates the webhook is a conversion webhook
	CRDConversion
	//APIServiceWebhook indicates the webhook is an extension API server
	APIService
)

func SetupCertRotator(mgr manager.Manager, options Options) (chan struct{}, error) {
	options.Default()

	// Make sure certs are generated and valid if cert rotation is enabled.
	setupFinished := make(chan struct{})
	klog.Info("setting up cert rotator")
	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		CertDir:        options.CertDir,
		CAName:         options.CAName,
		CAOrganization: options.CAOrganization,
		DNSName:        fmt.Sprintf("%s.%s.svc", options.ServiceName, options.Namespace),
		IsReady:        setupFinished,
		Webhooks:       convertWebhooks(options.Webhooks),
	})
	if err != nil {
		klog.Error(err, "unable to setup cert rotator")
		return nil, err
	}

	return setupFinished, nil
}

func convertWebhooks(webhooks []WebhookInfo) []rotator.WebhookInfo {
	result := make([]rotator.WebhookInfo, len(webhooks))

	for i := range webhooks {
		result[i] = rotator.WebhookInfo{
			Name: webhooks[i].Name,
			Type: convertWebhookType(webhooks[i].Type),
		}
	}

	return result
}

func convertWebhookType(typ WebhookType) rotator.WebhookType {
	switch typ {
	case Validating:
		return rotator.Validating
	case Mutating:
		return rotator.Mutating
	case CRDConversion:
		return rotator.CRDConversion
	case APIService:
		return rotator.APIService
	default:
		panic("not supported webhook type")
	}
}
