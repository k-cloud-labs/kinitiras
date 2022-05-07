package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func Response(allowed bool) admission.Response {
	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: allowed,
		},
	}
}

func ResponseStatus(allowed bool, status, msg string) admission.Response {
	r := Response(allowed)
	r.Result = &metav1.Status{
		Status:  status,
		Message: msg,
	}
	return r
}

func ResponseFailure(allowed bool, msg string) admission.Response {
	return ResponseStatus(allowed, metav1.StatusFailure, msg)
}
