package webhook

import (
	"context"
	"net/http"

	"github.com/k-cloud-labs/pkg/util/validatemanager"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ValidatingAdmission struct {
	decoder         *admission.Decoder
	validateManager validatemanager.ValidateManager
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj, oldObj, err := decodeObj(v.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	result, err := v.validateManager.ApplyValidatePolicies(obj, oldObj, admissionv1.Delete)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !result.Valid {
		return admission.Denied(result.Reason)
	}

	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (a *ValidatingAdmission) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

func NewValidatingAdmissionHandler(validateManager validatemanager.ValidateManager) webhook.AdmissionHandler {
	return &ValidatingAdmission{
		validateManager: validateManager,
	}
}
