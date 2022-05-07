package webhook

import (
	"context"
	"net/http"

	pkgadmission "github.com/k-cloud-labs/kinitiras/pkg/admission"
	"github.com/k-cloud-labs/pkg/util/validatemanager"
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

	result, err := v.validateManager.ApplyValidatePolicies(obj, oldObj, req.Operation)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !result.Valid {
		return pkgadmission.ResponseFailure(false, result.Reason)
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
