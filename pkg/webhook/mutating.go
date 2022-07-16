package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/overridemanager"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type MutatingAdmission struct {
	decoder         *admission.Decoder
	overrideManager overridemanager.OverrideManager
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}
var _ admission.DecoderInjector = &MutatingAdmission{}

func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj, _, err := decodeObj(a.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	newObj := obj.DeepCopy()

	cops, ops, err := a.overrideManager.ApplyOverridePolicies(newObj, req.Operation)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if klog.V(4).Enabled() {
		opBytes, err := ops.MarshalJSON()
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		copBytes, err := cops.MarshalJSON()
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		klog.V(4).InfoS("override policy applied.", "resource", klog.KObj(obj), utils.AppliedOverrides, string(opBytes), utils.AppliedClusterOverrides, string(copBytes))
	} else {
		klog.InfoS("override policy applied.", "resource", klog.KObj(obj))
	}

	patchedObj, err := json.Marshal(newObj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, patchedObj)
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (a *MutatingAdmission) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

func NewMutatingAdmissionHandler(overrideManager overridemanager.OverrideManager) webhook.AdmissionHandler {
	return &MutatingAdmission{
		overrideManager: overrideManager,
	}
}

func decodeObj(decoder *admission.Decoder, req admission.Request) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	var (
		obj    = &unstructured.Unstructured{}
		oldObj *unstructured.Unstructured
	)

	switch req.Operation {
	case admissionv1.Create:
		err := decoder.Decode(req, obj)
		if err != nil {
			return nil, nil, err
		}
	case admissionv1.Update:
		oldObj = obj.DeepCopy()
		err := decoder.DecodeRaw(req.Object, obj)
		if err != nil {
			return nil, nil, err
		}
		err = decoder.DecodeRaw(req.OldObject, oldObj)
		if err != nil {
			return obj, nil, err
		}
	case admissionv1.Delete:
		// In reference to PR: https://github.com/kubernetes/kubernetes/pull/76346
		// OldObject contains the object being deleted
		err := decoder.DecodeRaw(req.OldObject, obj)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, errors.New("unsupported operation")
	}

	return obj, oldObj, nil
}
