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
	"strings"
	"time"

	"github.com/danilonicioka/helmify"
	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/danilonicioka/helmify/pkg/helm"
	"github.com/danilonicioka/helmify/web"
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
	// If a Kubernetes service is named "helmify", it injects HELMIFY_PORT=tcp://...
	if strings.HasPrefix(port, "tcp://") {
		port = ""
	}
	
	if port == "" {
		port = os.Getenv("PORT") // Fallback to standard PORT
	}
	if port == "" || strings.HasPrefix(port, "tcp://") {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/generate", handleGenerate)
	mux.HandleFunc("/v1/generate-wizard", handleGenerateWizard)
	mux.HandleFunc("/v1/preview-wizard", handlePreviewWizard)
	mux.HandleFunc("/v1/defaults", handleDefaults)
	mux.HandleFunc("/v1/subcomponents", handleSubcomponents)
	mux.HandleFunc("/v1/preview", handlePreview)
	mux.HandleFunc("/v1/download", handleDownload)
	mux.HandleFunc("/wizard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.WizardHTML)
	})
	mux.HandleFunc("/wizard/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.WizardHTML)
	})
	mux.HandleFunc("/instructions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.InstructionsHTML)
	})
	mux.HandleFunc("/instructions/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.InstructionsHTML)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/converter", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.ConverterHTML)
	})
	mux.HandleFunc("/converter/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.ConverterHTML)
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
		logrus.WithField("port", port).Info("Starting Helmify")
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
	}).Info("Generating standard chart via extractor")

	params, err := helm.ExtractWizardParams(r.Body, conf)
	if err != nil {
		logrus.WithError(err).Error("Extraction failed")
		sendError(w, fmt.Sprintf("Failed to extract data: %v", err), http.StatusInternalServerError)
		return
	}

	files, err := helm.GenerateWizardChart(params)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate standard chart")
		sendError(w, fmt.Sprintf("Failed to generate chart: %v", err), http.StatusInternalServerError)
		return
	}

	valuesOnly := r.URL.Query().Get("values-only") == "true"
	if valuesOnly {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Content-Disposition", `attachment; filename="values.yaml"`)
		if valContent, ok := files["values.yaml"]; ok {
			w.Write(valContent)
		}
	} else {
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Disposition", `attachment; filename="chart.tar.gz"`)

		if err := helm.WriteTarGz(files, conf.ChartName, w); err != nil {
			logrus.WithError(err).Error("TarGz streaming failed")
		}
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
	}).Info("Generating preview via extractor")

	params, err := helm.ExtractWizardParams(r.Body, conf)
	if err != nil {
		logrus.WithError(err).Error("Extraction failed")
		sendError(w, fmt.Sprintf("Failed to extract data: %v", err), http.StatusInternalServerError)
		return
	}

	files, err := helm.GenerateWizardChart(params)
	if err != nil {
		logrus.WithError(err).Error("Preview execution failed")
		sendError(w, fmt.Sprintf("Failed to generate preview: %v", err), http.StatusInternalServerError)
		return
	}

	preview := make(map[string]string)
	for name, content := range files {
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
	w.Header().Set("Content-Disposition", `attachment; filename="chart.tar.gz"`)

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
		cv := strings.TrimSpace(helmify.ChartVersion)
		htmlStr := strings.Replace(string(web.HomeHTML), "{{CHART_VERSION}}", cv, 1)
		w.Write([]byte(htmlStr))
		return
	}
	http.NotFound(w, r)
}
