package helm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
				
				// Extract Resources
				resources, resFound, _ := unstructured.NestedMap(container, "resources")
				if resFound {
					resParams := &ResourceParams{}
					if limits, ok, _ := unstructured.NestedMap(resources, "limits"); ok && len(limits) > 0 {
						resParams.Limits = limits
					}
					if requests, ok, _ := unstructured.NestedMap(resources, "requests"); ok && len(requests) > 0 {
						resParams.Requests = requests
					}
					if resParams.Limits != nil || resParams.Requests != nil {
						depParams.Resources = resParams
					}
				}

				// Extract Probes
				if p, ok, _ := unstructured.NestedMap(container, "startupProbe"); ok && len(p) > 0 {
					depParams.StartupProbe = p
				}
				if p, ok, _ := unstructured.NestedMap(container, "livenessProbe"); ok && len(p) > 0 {
					depParams.LivenessProbe = p
				}
				if p, ok, _ := unstructured.NestedMap(container, "readinessProbe"); ok && len(p) > 0 {
					depParams.ReadinessProbe = p
				}

				// Extract Persistence from volumeMounts
				mounts, ok, _ := unstructured.NestedSlice(container, "volumeMounts")
				if ok && len(mounts) > 0 {
					for _, m := range mounts {
						mount := m.(map[string]interface{})
						name, _, _ := unstructured.NestedString(mount, "name")
						if !strings.HasPrefix(name, "kube-api") && !strings.Contains(name, "default-token") {
							path, _, _ := unstructured.NestedString(mount, "mountPath")
							depParams.Persistence.Enabled = true
							depParams.Persistence.MountPath = path
							break // Take the first meaningful volume for the simple model
						}
					}
				}
			}

			if affinity, ok, _ := unstructured.NestedMap(obj.Object, "spec", "template", "spec", "affinity"); ok && len(affinity) > 0 {
				// Seamlessly modernize legacy labels to our new standard component pattern
				if b, err := json.Marshal(affinity); err == nil {
					b = bytes.ReplaceAll(b, []byte("app.kubernetes.io/name"), []byte("app.kubernetes.io/component"))
					var modernAffinity map[string]interface{}
					if json.Unmarshal(b, &modernAffinity) == nil {
						depParams.Affinity = modernAffinity
					} else {
						depParams.Affinity = affinity
					}
				} else {
					depParams.Affinity = affinity
				}
			}
			if nodeSelector, ok, _ := unstructured.NestedMap(obj.Object, "spec", "template", "spec", "nodeSelector"); ok && len(nodeSelector) > 0 {
				depParams.NodeSelector = nodeSelector
			}
			if tolerations, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "tolerations"); ok && len(tolerations) > 0 {
				depParams.Tolerations = tolerations
			}

			// Extract Ephemeral Persistence (emptyDir)
			volumes, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "volumes")
			if ok && len(volumes) > 0 {
				for _, v := range volumes {
					vol := v.(map[string]interface{})
					if _, ok, _ := unstructured.NestedMap(vol, "emptyDir"); ok {
						depParams.Persistence.Enabled = true
						depParams.Persistence.Ephemeral = true
						break
					}
					if pvc, ok, _ := unstructured.NestedMap(vol, "persistentVolumeClaim"); ok {
						depParams.Persistence.Enabled = true
						depParams.Persistence.Ephemeral = false
						if claimName, _, _ := unstructured.NestedString(pvc, "claimName"); claimName != "" {
							// Try to use claimName if possible, though standard wizard model doesn't explicitly pass it unless needed
						}
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
				if targetPortInt, ok := portMap["targetPort"].(int64); ok {
					depParams.Service.TargetPort = int(targetPortInt)
				} else if targetPortFloat, ok := portMap["targetPort"].(float64); ok {
					depParams.Service.TargetPort = int(targetPortFloat)
				} else if targetPortStr, ok := portMap["targetPort"].(string); ok {
					// In some legacy K8s manifests, targetPort is occasionally an IANA string (e.g. "http")
					// The simplest standard fallback for the wizard is to leave it 0 which triggers the 8080 default
					_ = targetPortStr
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

			if host == "" || strings.HasSuffix(host, config.GlobalEnvConfig.DefaultDomain) || strings.Contains(host, "apps.ocp-") {
				depParams.Route.Default.Enabled = true
				if host != "" {
					depParams.Route.Default.Host = host
				}
			} else if strings.HasSuffix(host, config.GlobalEnvConfig.InternalDomain) {
				depParams.Route.Internal.Enabled = true
				depParams.Route.Internal.Host = host
			} else if strings.HasSuffix(host, config.GlobalEnvConfig.ExternalDomain) {
				depParams.Route.External.Enabled = true
				depParams.Route.External.Host = host
			} else {
				// Fallback to external if unknown
				depParams.Route.External.Enabled = true
				depParams.Route.External.Host = host
			}
			if path != "" {
				depParams.Route.Path = path
			}
			params.Deployments[compName] = depParams

		case "HorizontalPodAutoscaler":
			if compName == "" {
				continue
			}
			depParams := params.Deployments[compName]
			
			hpaMap := make(map[string]interface{})
			hpaMap["enabled"] = true
			
			if min, found, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicas"); found {
				hpaMap["minReplicas"] = int(min)
			}
			if max, found, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicas"); found {
				hpaMap["maxReplicas"] = int(max)
			}
			if metrics, found, _ := unstructured.NestedSlice(obj.Object, "spec", "metrics"); found {
				hpaMap["metrics"] = metrics
			}
			if behavior, found, _ := unstructured.NestedMap(obj.Object, "spec", "behavior"); found {
				hpaMap["behavior"] = behavior
			}
			
			depParams.Hpa = hpaMap
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
