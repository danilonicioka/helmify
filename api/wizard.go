package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

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
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, params.ChartName))

	if err := helm.WriteTarGz(files, params.ChartName, w); err != nil {
		logrus.WithError(err).Error("Failed to write tar.gz stream")
	}
}
