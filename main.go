package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jukie/k8s-secret-injector/mutator"
	klog "k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)

	var imagePullSecretName, registry string
	var ok bool
	if imagePullSecretName, ok = os.LookupEnv("IMAGE_PULL_SECRET"); !ok {
		klog.Fatalln("Env var IMAGE_PULL_SECRET is unset, exiting.")
	}
	if registry, ok = os.LookupEnv("IMAGE_REGISTRY"); !ok {
		klog.Fatalln("Env var IMAGE_REGISTRY is unset, exiting.")
	}
	c := mutator.NewController(imagePullSecretName, registry)

	handler := http.NewServeMux()
	handler.Handle("/mutate", c)
	server := http.Server{
		Addr:    ":8443",
		Handler: handler,
	}

	go func() {
		klog.Info("Starting server...")
		if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	<-stopCh
	klog.Infoln("Got OS shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown(context.Background())

}
