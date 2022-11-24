package webhook

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/k-cloud-labs/pkg/utils/interrupter"
	"github.com/k-cloud-labs/pkg/utils/validatemanager"

	pkgadmission "github.com/k-cloud-labs/kinitiras/pkg/admission"
)

type ValidatingAdmission struct {
	decoder                  *admission.Decoder
	validateManager          validatemanager.ValidateManager
	policyInterrupterManager interrupter.PolicyInterrupter
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj, oldObj, err := decodeObj(v.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// if obj is known policy, then run policy interrupter
	err = v.policyInterrupterManager.OnValidating(obj, oldObj, req.Operation)
	if err != nil {
		return admission.Denied(err.Error())
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

func NewValidatingAdmissionHandler(validateManager validatemanager.ValidateManager, policyInterrupterManager interrupter.PolicyInterrupterManager) webhook.AdmissionHandler {
	return &ValidatingAdmission{
		validateManager:          validateManager,
		policyInterrupterManager: policyInterrupterManager,
	}
}
