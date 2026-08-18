package helm

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/danilonicioka/helmify/pkg/decoder"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractWizardParams(reader io.Reader, conf config.Config) (WizardParams, error) {
	stop := make(chan struct{})
	defer close(stop)

	streamedObjects := decoder.Decode(stop, reader)

	params := WizardParams{
		ChartName:    conf.ChartName,
		DevRepoURL:   conf.DevRepoURL,
		Deployments:  make(map[string]DeploymentParams),
		GlobalConfig: make(map[string]string),
	}

	var objects []*unstructured.Unstructured
	for obj := range streamedObjects {
		objects = append(objects, obj)
	}

	// Pass 1: Discover all Deployments/StatefulSets/DaemonSets
	var compNames []string
	for _, obj := range objects {
		kind := obj.GetKind()
		if kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
			name := obj.GetName()
			compNames = append(compNames, name)

			depParams := DeploymentParams{
				Cm:     make(map[string]string),
				Secret: make(map[string]string),
			}

			// Extract Image, Replicas
			replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
			if err == nil && found {
				r := int(replicas)
				depParams.Replicas = &r
			}

			containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
			if err == nil && found && len(containers) > 0 {
				container := containers[0].(map[string]interface{})
				image, _, _ := unstructured.NestedString(container, "image")
				if image != "" {
					parts := strings.SplitN(image, ":", 2)
					depParams.Image.Repository = parts[0]
					if len(parts) > 1 {
						depParams.Image.Tag = parts[1]
					} else {
						depParams.Image.Tag = "latest"
					}
				}
			}

			// Extract annotations/labels
			labels := obj.GetLabels()
			if runtime, ok := labels["app.openshift.io/runtime"]; ok {
				depParams.Runtime = runtime
			}

			annotations := obj.GetAnnotations()
			if overview, ok := annotations["console.alpha.openshift.io/overview-app-route"]; ok {
				depParams.OverviewAppRoute = overview
			}

			params.Deployments[name] = depParams
		}
	}

	// Helper to find the closest matching component name
	findComponent := func(objName string, labels map[string]string) string {
		if comp, ok := labels["app.kubernetes.io/component"]; ok {
			if _, exists := params.Deployments[comp]; exists {
				return comp
			}
		}
		for _, c := range compNames {
			if strings.Contains(objName, c) {
				return c
			}
		}
		if len(compNames) > 0 {
			return compNames[0]
		}
		return ""
	}

	// Pass 2: Map Services, ConfigMaps, Secrets, Routes
	for _, obj := range objects {
		kind := obj.GetKind()
		name := obj.GetName()
		compName := findComponent(name, obj.GetLabels())

		switch kind {
		case "Service":
			if compName == "" {
				continue
			}
			depParams := params.Deployments[compName]
			ports, found, err := unstructured.NestedSlice(obj.Object, "spec", "ports")
			if err == nil && found && len(ports) > 0 {
				portMap := ports[0].(map[string]interface{})
				if portInt, ok := portMap["port"].(int64); ok {
					depParams.Service.Port = int(portInt)
				} else if portFloat, ok := portMap["port"].(float64); ok {
					depParams.Service.Port = int(portFloat)
				}
			}
			params.Deployments[compName] = depParams

		case "ConfigMap":
			data, found, err := unstructured.NestedMap(obj.Object, "data")
			if err == nil && found {
				if compName != "" {
					depParams := params.Deployments[compName]
					for k, v := range data {
						depParams.Cm[k] = fmt.Sprintf("%v", v)
					}
					params.Deployments[compName] = depParams
				} else {
					for k, v := range data {
						params.GlobalConfig[k] = fmt.Sprintf("%v", v)
					}
				}
			}

		case "Secret":
			if compName == "" {
				continue // Global secrets aren't strongly supported by standard model yet
			}
			depParams := params.Deployments[compName]

			stringData, foundStr, _ := unstructured.NestedMap(obj.Object, "stringData")
			if foundStr {
				for k, v := range stringData {
					depParams.Secret[k] = fmt.Sprintf("%v", v)
				}
			}
			data, found, _ := unstructured.NestedMap(obj.Object, "data")
			if found {
				for k, v := range data {
					if strVal, ok := v.(string); ok {
						decoded, err := base64.StdEncoding.DecodeString(strVal)
						if err == nil {
							depParams.Secret[k] = string(decoded)
						} else {
							depParams.Secret[k] = strVal
						}
					}
				}
			}
			params.Deployments[compName] = depParams

		case "Route":
			if compName == "" {
				continue
			}
			depParams := params.Deployments[compName]

			host, _, _ := unstructured.NestedString(obj.Object, "spec", "host")
			path, _, _ := unstructured.NestedString(obj.Object, "spec", "path")

			if strings.Contains(host, "ocp-dev") || host == "" {
				depParams.Route.Default.Enabled = true
				if host != "" {
					depParams.Route.Default.Host = host
				}
			} else if strings.Contains(host, "int") {
				depParams.Route.Internal.Enabled = true
				depParams.Route.Internal.Host = host
			} else {
				depParams.Route.External.Enabled = true
				depParams.Route.External.Host = host
			}
			if path != "" {
				depParams.Route.Path = path
			}
			params.Deployments[compName] = depParams
		}
	}

	// Auto-detect type and normalize naming if single
	if len(params.Deployments) == 1 {
		params.Type = "single"
		// If there is only one component, the wizard expects it to be named the same as the chart
		var oldKey string
		for k := range params.Deployments {
			oldKey = k
			break
		}
		if oldKey != params.ChartName {
			params.Deployments[params.ChartName] = params.Deployments[oldKey]
			delete(params.Deployments, oldKey)
		}
	} else if len(params.Deployments) > 1 {
		params.Type = "multi"
	} else {
		return params, fmt.Errorf("no valid Kubernetes Deployments found in input (ensure your manifests are correct and contain at least one Deployment)")
	}

	return params, nil
}
