package patch

import (
	"encoding/json"
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func PodMutator(review admissionv1.AdmissionReview, pod *v1.Pod, imagePullSecret, registry string) (admissionv1.AdmissionReview, error) {
	response := admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}
	if !isCustomRegistryPod(pod, registry) {
		response.Result = &metav1.Status{
			Message: "not using custom registry",
		}
		review.Response = &response
		return review, nil
	}

	if hasImagePullSecret(pod, imagePullSecret) {
		response.Result = &metav1.Status{
			Message: "already has imagePullSecret",
		}
		review.Response = &response
		return review, nil
	}

	return addImagePullSecretPatch(pod, imagePullSecret, registry, review, response)

}

func isCustomRegistryPod(pod *v1.Pod, registry string) bool {
	for _, container := range pod.Spec.Containers {
		if strings.HasPrefix(container.Image, registry) {
			return true
		}
	}
	return false
}

func hasImagePullSecret(pod *v1.Pod, injectedPullSecret string) bool {
	for _, pullSecret := range pod.Spec.ImagePullSecrets {
		if pullSecret.Name == injectedPullSecret {
			return true
		}
	}
	return false
}

func addImagePullSecretPatch(pod *v1.Pod, imagePullSecretName, registry string, review admissionv1.AdmissionReview, response admissionv1.AdmissionResponse) (admissionv1.AdmissionReview, error) {
	needsPatch := false
	for _, container := range append(pod.Spec.Containers, pod.Spec.InitContainers...) {
		if !strings.HasPrefix(container.Image, registry) {
			continue
		}
		if hasImagePullSecret(pod, imagePullSecretName) {
			break
		}
		needsPatch = true
		break
	}
	if needsPatch {
		klog.Infof("Patching imagePullSecret for Pod %s from Namespace %s", pod.Name, pod.Namespace)
		imagePull := []v1.LocalObjectReference{{Name: imagePullSecretName}}
		patch := []patchOperation{
			{
				Op:    "add",
				Path:  "/spec/imagePullSecrets",
				Value: imagePull,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return review, fmt.Errorf("failed to marshal patch response: %v", err)
		}
		response.Patch = patchBytes
		patchType := admissionv1.PatchTypeJSONPatch
		response.PatchType = &patchType
		review.Response = &response
	}
	return review, nil
}
