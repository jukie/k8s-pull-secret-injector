package mutator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/jukie/k8s-secret-injector/patch"
	"k8s.io/apimachinery/pkg/runtime/schema"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	klog "k8s.io/klog/v2"
)

type Mutator struct {
	Clientset           kubernetes.Interface
	Decoder             runtime.Decoder
	ImagePullSecretName string
	Registry            string
}

func NewController(imagePullSecretName, registry string) *Mutator {
	clientset, err := newClient()
	if err != nil {
		klog.Fatalln(err)
	}

	c := &Mutator{
		ImagePullSecretName: imagePullSecretName,
		Registry:            registry,
		Clientset:           clientset,
		Decoder:             serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer(),
	}
	return c
}

func newClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	kubeCfgPath := os.Getenv("KUBECONFIGPATH")
	if kubeCfgPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}
	return clientset, nil
}

func (c *Mutator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make sure request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Check content type is JSON
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Invalid content type", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		e := fmt.Errorf("could not read request body: %v", err)
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}

	var rev admissionv1.AdmissionReview
	if _, _, err := c.Decoder.Decode(body, nil, &rev); err != nil {
		e := fmt.Errorf("could not deserialize request: %v", err)
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}

	pod, err := c.getPodFromAdmissionRequest(&rev)
	if err != nil {
		e := fmt.Sprintf("Failed to unmarshal AdmissionReview: %v", err)
		klog.Errorln(e)
		http.Error(w, e, http.StatusBadRequest)
		return
	}

	if rev, err = patch.PodMutator(rev, pod, c.ImagePullSecretName, c.Registry); err != nil {
		e := fmt.Sprintf("Failed to mutate pod: %v", err)
		klog.Errorln(e)
		http.Error(w, e, http.StatusBadRequest)
		return
	}

	if err := writeResponse(w, rev); err != nil {
		e := fmt.Sprintf("failed to write response: %v", err)
		klog.Errorln(e)
		http.Error(w, e, http.StatusInternalServerError)
		return
	}
}
func (c *Mutator) getPodFromAdmissionRequest(rev *admissionv1.AdmissionReview) (*v1.Pod, error) {
	req := rev.Request
	pod := &v1.Pod{}
	_, _, err := c.Decoder.Decode(req.Object.Raw, &schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw object: %v", err)
	}
	return pod, nil
}

func writeResponse(w http.ResponseWriter, rev admissionv1.AdmissionReview) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// The resulting byteslice has invalid utf8 characters and the k8s api server complains
	// bytes, err := rev.Marshal()

	bytes, err := json.Marshal(&rev)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %v", err)
	}

	if _, err := w.Write(bytes); err != nil {
		return fmt.Errorf("failed to write response: %v", err)
	}

	return nil
}
