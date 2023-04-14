package patch_test

import (
	"testing"

	"github.com/jukie/k8s-secret-injector/patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPodMutator(t *testing.T) {
	tests := []struct {
		name            string
		pod             *v1.Pod
		imagePullSecret string
		registry        string
		expectPatch     bool
	}{
		{
			name:            "No custom registry",
			pod:             createTestPod("default-image"),
			imagePullSecret: "test-secret",
			registry:        "test-registry",
			expectPatch:     false,
		},
		{
			name:            "Custom registry, no existing imagePullSecret",
			pod:             createTestPod("test-registry/test-image"),
			imagePullSecret: "test-secret",
			registry:        "test-registry",
			expectPatch:     true,
		},
		{
			name:            "Custom registry, existing imagePullSecret",
			pod:             createTestPodWithSecret("test-registry/test-image", "test-secret"),
			imagePullSecret: "test-secret",
			registry:        "test-registry",
			expectPatch:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review := createTestAdmissionReview(tt.pod)

			result, err := patch.PodMutator(review, tt.pod, tt.imagePullSecret, tt.registry)
			require.NoError(t, err)

			if tt.expectPatch {
				assert.NotNil(t, result.Response.PatchType)
				assert.NotNil(t, result.Response.Patch)
			} else {
				assert.Nil(t, result.Response.PatchType)
				assert.Nil(t, result.Response.Patch)
			}
		})
	}
}

func createTestPod(image string) *v1.Pod {
	return &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: image,
				},
			},
		},
	}
}

func createTestPodWithSecret(image, secret string) *v1.Pod {
	pod := createTestPod(image)
	pod.Spec.ImagePullSecrets = []v1.LocalObjectReference{
		{Name: secret},
	}
	return pod
}

func createTestAdmissionReview(pod *v1.Pod) admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "12345",
			Object: runtime.RawExtension{
				Object: pod,
			},
		},
	}
}
