package mutator_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jukie/k8s-secret-injector/mutator"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		requestBody    []byte
		expectedStatus int
	}{
		{
			name:           "Invalid method",
			method:         http.MethodGet,
			contentType:    "application/json",
			requestBody:    []byte("{}"),
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid content type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			requestBody:    []byte("{}"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Successful mutation",
			method:         http.MethodPost,
			contentType:    "application/json",
			requestBody:    createAdmissionReviewRequestBody(t),
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestMutator()

			req, err := http.NewRequest(tt.method, "", bytes.NewBuffer(tt.requestBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", tt.contentType)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(m.ServeHTTP)

			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func createAdmissionReviewRequestBody(t *testing.T) []byte {
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	raw, err := json.Marshal(pod)
	require.NoError(t, err)

	req := admissionv1.AdmissionRequest{
		UID: "12345",
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}

	ar := admissionv1.AdmissionReview{
		Request: &req,
	}

	body, err := json.Marshal(ar)
	require.NoError(t, err)

	return body
}

func createTestMutator() *mutator.Mutator {
	clientset := fake.NewSimpleClientset()

	decoder := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

	return &mutator.Mutator{
		Clientset:           clientset,
		Decoder:             decoder,
		ImagePullSecretName: "test-secret",
		Registry:            "test-registry",
	}
}
