package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/arttor/helmify/pkg/app"
	"github.com/arttor/helmify/pkg/config"
	"github.com/arttor/helmify/pkg/helm"
	"github.com/arttor/helmify/pkg/translator/k8smanifest"
	"github.com/sirupsen/logrus"
)

func init() {
	// Use JSON formatter for logs - standard for OpenShift/Kubernetes
	logrus.SetFormatter(&logrus.JSONFormatter{})
	
	// Set log level from environment
	levelStr := os.Getenv("HELMIFY_LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
}

func main() {
	port := os.Getenv("HELMIFY_PORT")
	if port == "" {
		port = os.Getenv("PORT") // Fallback to standard PORT
	}
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/generate", handleGenerate)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	// Serve the UI for all other paths
	mux.Handle("/", getUIHandler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logrus.WithField("port", port).Info("Starting Helmify API")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Server failed to start")
		}
	}()

	<-done
	logrus.Info("Server Stopping...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Fatal("Server Shutdown Failed")
	}
	logrus.Info("Server Exited Properly")
}

// errorResponse represents a structured JSON error
type errorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

func sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse{
		Error:  message,
		Status: code,
	})
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conf := config.Config{
		ChartName: r.Header.Get("X-Chart-Name"),
	}
	if conf.ChartName == "" {
		conf.ChartName = "chart"
	}

	// Simple header parsing for booleans
	conf.Crd, _ = strconv.ParseBool(r.Header.Get("X-Crd"))
	conf.CertManagerAsSubchart, _ = strconv.ParseBool(r.Header.Get("X-Cert-Manager-Subchart"))
	conf.CertManagerInstallCRD, _ = strconv.ParseBool(r.Header.Get("X-Cert-Manager-Install-Crd"))
	conf.AddWebhookOption, _ = strconv.ParseBool(r.Header.Get("X-Add-Webhook-Option"))
	conf.OptionalCRDs, _ = strconv.ParseBool(r.Header.Get("X-Optional-Crds"))
	conf.CertManagerVersion = r.Header.Get("X-Cert-Manager-Version")
	if conf.CertManagerVersion == "" {
		conf.CertManagerVersion = "v1.11.0"
	}

	logrus.WithFields(logrus.Fields{
		"chart_name": conf.ChartName,
		"crd":        conf.Crd,
	}).Info("Generating chart")

	memOut := helm.NewMemoryOutput()
	engine := app.NewEngine(conf, memOut)
	trans := k8smanifest.New(conf, r.Body)

	if err := engine.Run(r.Context(), trans); err != nil {
		logrus.WithError(err).Error("Engine execution failed")
		sendError(w, fmt.Sprintf("Failed to generate chart: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, conf.ChartName))

	if err := memOut.ToTarGz(conf.ChartName, w); err != nil {
		logrus.WithError(err).Error("TarGz streaming failed")
		// Note: we might have already sent some data, so we can't reliably send a JSON error here
	}
}
