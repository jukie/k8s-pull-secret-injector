package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jukie/k8s-secret-injector/cmd/logger"
	"github.com/jukie/k8s-secret-injector/cmd/mutator"
)

func main() {
	err := logger.InitLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	var imagePullSecretName, registry string

	// Define flags
	flag.StringVar(&imagePullSecretName, "image-pull-secret", "", "Image pull secret to inject")
	flag.StringVar(&registry, "target-registry", "", "Image registry prefix to check during pod mutation")

	c := mutator.NewController(imagePullSecretName, registry)

	handler := http.NewServeMux()
	handler.Handle("/mutate", c)
	server := http.Server{
		Addr:    ":8443",
		Handler: handler,
	}

	go func() {
		logger.Log.Info("Starting server...")
		if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil {
			logger.Log.Fatal(fmt.Sprintf("Failed to start server: %v", err))
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	<-stopCh
	logger.Log.Info("Got OS shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown(context.Background())

}
