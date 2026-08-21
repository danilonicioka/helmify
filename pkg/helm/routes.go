package helm

import (
	"fmt"
	"strings"

	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/danilonicioka/helmify/pkg/processor"
)

// getRouteHostPrefix returns the hostname prefix for a given component.
func getRouteHostPrefix(chartName, compName, path string, isMulti bool) string {
	compKebab := processor.NormalizeComponentName(compName)
	chartKebab := processor.NormalizeComponentName(chartName)

	if !isMulti || compKebab == chartKebab {
		return chartKebab
	}

	// For multi-deployment, omit component suffix if it's a frontend/web component, or if a path is specified (not empty)
	isFrontend := compKebab == "app" || compKebab == "web" || compKebab == "frontend" ||
		compKebab == "app-emissor" || compKebab == "bff-emissor" ||
		strings.Contains(compKebab, "web") || strings.Contains(compKebab, "frontend")
	if isFrontend || path != "" {
		return chartKebab
	}

	if strings.HasPrefix(compKebab, chartKebab+"-") {
		return compKebab
	}
	return chartKebab + "-" + compKebab
}

// computeRouteHosts returns the default, internal, and external hostnames.
func computeRouteHosts(chartName, compName, path string, isMulti bool) (string, string, string) {
	prefix := getRouteHostPrefix(chartName, compName, path, isMulti)
	defaultHost := fmt.Sprintf("%s.%s", prefix, strings.TrimPrefix(config.GlobalEnvConfig.DefaultDomain, "."))
	
	internalDomain := config.GlobalEnvConfig.InternalDomain
	var internalHost string
	if strings.HasPrefix(internalDomain, "-") {
		internalHost = fmt.Sprintf("%s%s", prefix, internalDomain)
	} else if strings.HasPrefix(internalDomain, ".") {
		internalHost = fmt.Sprintf("%s%s", prefix, internalDomain)
	} else {
		internalHost = fmt.Sprintf("%s.%s", prefix, internalDomain)
	}

	externalDomain := config.GlobalEnvConfig.ExternalDomain
	var externalHost string
	if strings.HasPrefix(externalDomain, "-") || strings.HasPrefix(externalDomain, ".") {
		externalHost = fmt.Sprintf("%s%s", prefix, externalDomain)
	} else {
		externalHost = fmt.Sprintf("%s.%s", prefix, externalDomain)
	}
	
	return defaultHost, internalHost, externalHost
}
