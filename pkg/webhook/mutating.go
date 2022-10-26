package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/interrupter"
	"github.com/k-cloud-labs/pkg/utils/overridemanager"
)

type MutatingAdmission struct {
	decoder           *admission.Decoder
	overrideManager   overridemanager.OverrideManager
	policyInterrupter interrupter.PolicyInterrupter
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}
var _ admission.DecoderInjector = &MutatingAdmission{}

func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj, oldObj, err := decodeObj(a.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	newObj := obj.DeepCopy()
	if req.Operation != admissionv1.Delete {
		// if obj is known policy, then run policy interrupter
		patches, err := a.policyInterrupter.OnMutating(newObj, oldObj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if len(patches) != 0 {
			// patch data
			patchedObj, err := json.Marshal(newObj)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}

			return admission.PatchResponseFromRaw(req.Object.Raw, patchedObj)
		}
	}

	if klog.V(7).Enabled() {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "\t")
		if err := enc.Encode(obj); err != nil {
			klog.ErrorS(err, "encode")
		}

		klog.V(4).InfoS("override obj", "obj", buf.String())
	}

	cops, ops, err := a.overrideManager.ApplyOverridePolicies(newObj, oldObj, req.Operation)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if klog.V(4).Enabled() {
		var opBytes, copBytes []byte
		if ops != nil {
			opBytes, err = ops.MarshalJSON()
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}
		if cops != nil {
			copBytes, err = cops.MarshalJSON()
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
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

func NewMutatingAdmissionHandler(overrideManager overridemanager.OverrideManager, policyInterrupter interrupter.PolicyInterrupter) webhook.AdmissionHandler {
	return &MutatingAdmission{
		overrideManager:   overrideManager,
		policyInterrupter: policyInterrupter,
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
