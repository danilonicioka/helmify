package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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

//go:embed home.html
var homeHTML []byte

//go:embed instructions.html
var instructionsHTML []byte

//go:embed converter.html
var converterHTML []byte

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
	mux.HandleFunc("/v1/generate-wizard", handleGenerateWizard)
	mux.HandleFunc("/v1/preview-wizard", handlePreviewWizard)
	mux.HandleFunc("/v1/defaults", handleDefaults)
	mux.HandleFunc("/v1/preview", handlePreview)
	mux.HandleFunc("/v1/download", handleDownload)
	mux.HandleFunc("/wizard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(wizardHTML)
	})
	mux.HandleFunc("/wizard/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(wizardHTML)
	})
	mux.HandleFunc("/instructions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(instructionsHTML)
	})
	mux.HandleFunc("/instructions/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(instructionsHTML)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/converter", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(converterHTML)
	})
	mux.HandleFunc("/converter/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(converterHTML)
	})
	// Serve the portal homepage or other assets on /
	mux.HandleFunc("/", handleHomeOrAssets)

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

	conf := parseConfig(r)
	logrus.WithFields(logrus.Fields{
		"chart_name": conf.ChartName,
		"op":         "generate",
	}).Info("Generating chart")

	memOut := helm.NewMemoryOutput()
	memOut.DevRepoURL = conf.DevRepoURL
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
	}
}

func handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conf := parseConfig(r)
	logrus.WithFields(logrus.Fields{
		"chart_name": conf.ChartName,
		"op":         "preview",
	}).Info("Generating preview")

	memOut := helm.NewMemoryOutput()
	memOut.DevRepoURL = conf.DevRepoURL
	engine := app.NewEngine(conf, memOut)
	trans := k8smanifest.New(conf, r.Body)

	if err := engine.Run(r.Context(), trans); err != nil {
		logrus.WithError(err).Error("Preview execution failed")
		sendError(w, fmt.Sprintf("Failed to generate preview: %v", err), http.StatusInternalServerError)
		return
	}

	preview := make(map[string]string)
	for name, content := range memOut.Files {
		preview[name] = string(content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preview)
}

type downloadRequest struct {
	ChartName string            `json:"chartName"`
	Files     map[string]string `json:"files"`
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChartName == "" {
		req.ChartName = "chart"
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, req.ChartName))

	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range req.Files {
		var path string
		if name == ".gitlab-ci.yml" || name == "README.md" {
			path = name
		} else {
			path = filepath.Join("chart", name)
		}
		header := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			logrus.WithError(err).Error("Failed to write tar header")
			return
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			logrus.WithError(err).Error("Failed to write tar content")
			return
		}
	}
}

func parseConfig(r *http.Request) config.Config {
	conf := config.Config{
		ChartName: r.Header.Get("X-Chart-Name"),
	}
	if conf.ChartName == "" {
		conf.ChartName = "chart"
	}

	conf.Crd, _ = strconv.ParseBool(r.Header.Get("X-Crd"))
	conf.CertManagerAsSubchart, _ = strconv.ParseBool(r.Header.Get("X-Cert-Manager-Subchart"))
	conf.CertManagerInstallCRD, _ = strconv.ParseBool(r.Header.Get("X-Cert-Manager-Install-Crd"))
	conf.AddWebhookOption, _ = strconv.ParseBool(r.Header.Get("X-Add-Webhook-Option"))
	conf.OptionalCRDs, _ = strconv.ParseBool(r.Header.Get("X-Optional-Crds"))
	conf.GenerateAllTemplates, _ = strconv.ParseBool(r.Header.Get("X-Generate-All-Templates"))
	conf.CertManagerVersion = r.Header.Get("X-Cert-Manager-Version")
	if conf.CertManagerVersion == "" {
		conf.CertManagerVersion = "v1.11.0"
	}
	conf.DevRepoURL = r.Header.Get("X-Dev-Repo-Url")
	return conf
}

func handleHomeOrAssets(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(homeHTML)
		return
	}
	getUIHandler().ServeHTTP(w, r)
}
