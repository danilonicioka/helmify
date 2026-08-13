package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/arttor/helmify/pkg/helm"
	"github.com/sirupsen/logrus"
)

//go:embed index.html
var wizardHTML []byte

func handleGenerateWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params helm.WizardParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		logrus.WithError(err).Error("Failed to parse request body")
		sendError(w, fmt.Sprintf("Invalid JSON request: %v", err), http.StatusBadRequest)
		return
	}

	logrus.Infof("Generating Wizard chart: %s (%s)", params.ChartName, params.Type)

	files, err := helm.GenerateWizardChart(params)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate wizard chart")
		sendError(w, fmt.Sprintf("Failed to generate chart: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", `attachment; filename="chart.tar.gz"`)

	if err := helm.WriteTarGz(files, params.ChartName, w); err != nil {
		logrus.WithError(err).Error("Failed to write tar.gz stream")
	}
}

func handleDefaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chartType := r.URL.Query().Get("type")
	if chartType == "" {
		chartType = "single"
	}

	defaults, err := helm.GetModelDefaults(chartType)
	if err != nil {
		logrus.WithError(err).Error("Failed to retrieve defaults")
		sendError(w, fmt.Sprintf("Failed to retrieve defaults: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(defaults); err != nil {
		logrus.WithError(err).Error("Failed to encode defaults to JSON")
	}
}

func handlePreviewWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params helm.WizardParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		logrus.WithError(err).Error("Failed to parse request body")
		sendError(w, fmt.Sprintf("Invalid JSON request: %v", err), http.StatusBadRequest)
		return
	}

	files, err := helm.GenerateWizardChart(params)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate wizard chart preview")
		sendError(w, fmt.Sprintf("Failed to generate preview: %v", err), http.StatusInternalServerError)
		return
	}

	preview := make(map[string]string)
	for name, content := range files {
		preview[name] = string(content)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(preview); err != nil {
		logrus.WithError(err).Error("Failed to encode preview to JSON")
	}
}

func handleSubcomponents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	subcomponents := []string{}
	entries, err := os.ReadDir("models/subcomponents")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				subcomponents = append(subcomponents, e.Name())
			}
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(subcomponents); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
