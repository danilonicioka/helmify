package helm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/danilonicioka/helmify/pkg/decoder"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type VolumeMapping struct {
	MountPath  string
	SubPath    string
	SourceType  string // "configMap" or "secret"
	SourceName  string
	SidecarName string // set if mounted by a sidecar
}

func cleanMultilineString(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.Join(lines, "\n")
}

type EnvTarget struct {
	CompName string
	SidecarName string
	IsMain bool
	RefCount int
}

func ExtractWizardParams(reader io.Reader, conf config.Config) (WizardParams, error) {
	stop := make(chan struct{})
	defer close(stop)

	streamedObjects := decoder.Decode(stop, reader)

	params := WizardParams{
		ChartName:    conf.ChartName,
		DevRepoURL:   conf.DevRepoURL,
		Deployments:  make(map[string]DeploymentParams),
		GlobalConfig: make(map[string]string),
		GlobalSecret: make(map[string]string),
	}

	var objects []*unstructured.Unstructured
	for obj := range streamedObjects {
		objects = append(objects, obj)
	}
	logrus.Infof("Extracted %d objects", len(objects))
	envTracker := make(map[string]*EnvTarget)

	volMappings := make(map[string][]VolumeMapping)

	// Pass 1: Discover all Deployments/StatefulSets/DaemonSets
	var compNames []string
	for _, obj := range objects {
		kind := obj.GetKind()
		if kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" || kind == "CronJob" {
			name := obj.GetName()

			// Strip chart name prefix to cleanly determine component names (e.g. 'entremanas-api' -> 'api')
			cleanName := name
			if conf.ChartName != "" && strings.HasPrefix(cleanName, conf.ChartName+"-") {
				cleanName = strings.TrimPrefix(cleanName, conf.ChartName+"-")
			} else if conf.ChartName != "" && strings.HasPrefix(cleanName, conf.ChartName) {
				cleanName = strings.TrimPrefix(cleanName, conf.ChartName)
				cleanName = strings.TrimPrefix(cleanName, "-")
			}
			if cleanName != "" {
				name = cleanName
			}
			compNames = append(compNames, name)

			var depParams DeploymentParams
			if kind == "CronJob" {
				depParams = DeploymentParams{
					WorkloadType: "CronJob",
					Config:       &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
					Secrets:      &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
				}
				if schedule, found, _ := unstructured.NestedString(obj.Object, "spec", "schedule"); found {
					depParams.Schedule = schedule
				}
			} else {
				depParams = DeploymentParams{
					WorkloadType: kind,
					Config:       &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
					Secrets:      &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
				}
				// Extract Replicas for non-CronJobs
				replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
				if err == nil && found {
					r := int(replicas)
					depParams.Replicas = &r
				}
			}

			// Pod spec path differs for CronJobs
			podSpecPath := []string{"spec", "template", "spec"}
			if kind == "CronJob" {
				podSpecPath = []string{"spec", "jobTemplate", "spec", "template", "spec"}
			}

			// First, extract volumes and populate volSources
			volumesPath := append(podSpecPath, "volumes")
			volumes, vOk, _ := unstructured.NestedSlice(obj.Object, volumesPath...)
			volSources := make(map[string]struct{Type, Name string})
			if vOk && len(volumes) > 0 {
				for _, v := range volumes {
					vol := v.(map[string]interface{})
					name, _, _ := unstructured.NestedString(vol, "name")
					if cm, ok, _ := unstructured.NestedMap(vol, "configMap"); ok {
						cmName, _, _ := unstructured.NestedString(cm, "name")
						volSources[name] = struct{Type, Name string}{"configMap", cmName}
					} else if secret, ok, _ := unstructured.NestedMap(vol, "secret"); ok {
						secName, _, _ := unstructured.NestedString(secret, "secretName")
						volSources[name] = struct{Type, Name string}{"secret", secName}
					} else if _, ok, _ := unstructured.NestedMap(vol, "emptyDir"); ok {
						depParams.Persistence.Enabled = true
						depParams.Persistence.Ephemeral = true
					} else if _, ok, _ := unstructured.NestedMap(vol, "persistentVolumeClaim"); ok {
						depParams.Persistence.Enabled = true
						depParams.Persistence.Ephemeral = false
					}
				}
			}

			containersPath := append(podSpecPath, "containers")
			containers, found, err := unstructured.NestedSlice(obj.Object, containersPath...)
			if err == nil && found && len(containers) > 0 {
				container := containers[0].(map[string]interface{})
				populateContainerParams(&depParams, container, volSources, envTracker, obj.GetName())

				if len(containers) > 1 {
					if depParams.ExtraContainers == nil {
						depParams.ExtraContainers = make(map[string]*SidecarParams)
					}
					for i := 1; i < len(containers); i++ {
						if containerMap, ok := containers[i].(map[string]interface{}); ok {
							name, _, _ := unstructured.NestedString(containerMap, "name")
							if name == "" {
								name = fmt.Sprintf("sidecar-%d", i)
							}
							sidecarParams := &SidecarParams{
								Config:       &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
								Secrets:      &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
							}
							populateSidecarParams(sidecarParams, containerMap, volSources, envTracker, obj.GetName(), name)
							depParams.ExtraContainers[name] = sidecarParams
						}
					}
				}
			}

			initContainersPath := append(podSpecPath, "initContainers")
			initContainers, found, err := unstructured.NestedSlice(obj.Object, initContainersPath...)
			if err == nil && found && len(initContainers) > 0 {
				if depParams.InitContainers == nil {
					depParams.InitContainers = make(map[string]*SidecarParams)
				}
				for i := 0; i < len(initContainers); i++ {
					if containerMap, ok := initContainers[i].(map[string]interface{}); ok {
						name, _, _ := unstructured.NestedString(containerMap, "name")
						if name == "" {
							name = fmt.Sprintf("init-%d", i)
						}
						sidecarParams := &SidecarParams{
							Config:       &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
							Secrets:      &ConfigParams{Env: make(map[string]string), Files: make(map[string]CustomFileParams)},
						}
						populateSidecarParams(sidecarParams, containerMap, volSources, envTracker, obj.GetName(), name)
						depParams.InitContainers[name] = sidecarParams
					}
				}
			}

			affinityPath := append(podSpecPath, "affinity")
			if affinity, ok, _ := unstructured.NestedMap(obj.Object, affinityPath...); ok && len(affinity) > 0 {
				if b, err := json.Marshal(affinity); err == nil {
					b = bytes.ReplaceAll(b, []byte("app.kubernetes.io/name"), []byte("app.kubernetes.io/component"))
					var modernAffinity AffinityParams
					if json.Unmarshal(b, &modernAffinity) == nil {
						if depParams.Scheduling == nil { depParams.Scheduling = &SchedulingParams{} }
						depParams.Scheduling.Affinity = &modernAffinity
					}
				}
			}
			nodeSelectorPath := append(podSpecPath, "nodeSelector")
			if nodeSelector, ok, _ := unstructured.NestedMap(obj.Object, nodeSelectorPath...); ok && len(nodeSelector) > 0 {
				if depParams.Scheduling == nil { depParams.Scheduling = &SchedulingParams{} }
				depParams.Scheduling.NodeSelector = nodeSelector
			}
			tolerationsPath := append(podSpecPath, "tolerations")
			if tolerations, ok, _ := unstructured.NestedSlice(obj.Object, tolerationsPath...); ok && len(tolerations) > 0 {
				if depParams.Scheduling == nil { depParams.Scheduling = &SchedulingParams{} }
				depParams.Scheduling.Tolerations = tolerations
			}

			// Track volume mappings for Pass 2 (Custom Files routing)
			var currentVols []VolumeMapping
			if len(containers) > 0 {
				container := containers[0].(map[string]interface{})
				if mounts, mOk, _ := unstructured.NestedSlice(container, "volumeMounts"); mOk {
					for _, m := range mounts {
						mount := m.(map[string]interface{})
						name, _, _ := unstructured.NestedString(mount, "name")
						mountPath, _, _ := unstructured.NestedString(mount, "mountPath")
						subPath, _, _ := unstructured.NestedString(mount, "subPath")
						if src, found := volSources[name]; found {
							currentVols = append(currentVols, VolumeMapping{
								MountPath:  mountPath,
								SubPath:    subPath,
								SourceType: src.Type,
								SourceName: src.Name,
							})
						}
					}
				}
			}
			if len(containers) > 1 {
				for i := 1; i < len(containers); i++ {
					container := containers[i].(map[string]interface{})
					sidecarName, _, _ := unstructured.NestedString(container, "name")
					if sidecarName == "" {
						sidecarName = fmt.Sprintf("sidecar-%d", i)
					}
					if mounts, mOk, _ := unstructured.NestedSlice(container, "volumeMounts"); mOk {
						for _, m := range mounts {
							mount := m.(map[string]interface{})
							name, _, _ := unstructured.NestedString(mount, "name")
							mountPath, _, _ := unstructured.NestedString(mount, "mountPath")
							subPath, _, _ := unstructured.NestedString(mount, "subPath")
							if src, found := volSources[name]; found {
								currentVols = append(currentVols, VolumeMapping{
									MountPath:   mountPath,
									SubPath:     subPath,
									SourceType:  src.Type,
									SourceName:  src.Name,
									SidecarName: sidecarName,
								})
							}
						}
					}
				}
			}
			if len(initContainers) > 0 {
				for i := 0; i < len(initContainers); i++ {
					container := initContainers[i].(map[string]interface{})
					sidecarName, _, _ := unstructured.NestedString(container, "name")
					if sidecarName == "" {
						sidecarName = fmt.Sprintf("init-%d", i)
					}
					if mounts, mOk, _ := unstructured.NestedSlice(container, "volumeMounts"); mOk {
						for _, m := range mounts {
							mount := m.(map[string]interface{})
							name, _, _ := unstructured.NestedString(mount, "name")
							mountPath, _, _ := unstructured.NestedString(mount, "mountPath")
							subPath, _, _ := unstructured.NestedString(mount, "subPath")
							if src, found := volSources[name]; found {
								currentVols = append(currentVols, VolumeMapping{
									MountPath:   mountPath,
									SubPath:     subPath,
									SourceType:  src.Type,
									SourceName:  src.Name,
									SidecarName: sidecarName,
								})
							}
						}
					}
				}
			}
			volMappings[name] = currentVols

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
		if strings.Contains(objName, "-global") {
			return ""
		}
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
				if depParams.Service.Ports == nil {
					depParams.Service.Ports = make(map[string]struct {
						Port     int    `json:"port" yaml:"port"`
						Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
					})
				}
				for _, p := range ports {
					portMap := p.(map[string]interface{})
					name, _, _ := unstructured.NestedString(portMap, "name")
					protocol, _, _ := unstructured.NestedString(portMap, "protocol")
					
					var portVal int
					if portInt, ok := portMap["port"].(int64); ok {
						portVal = int(portInt)
					} else if portFloat, ok := portMap["port"].(float64); ok {
						portVal = int(portFloat)
					}

					if portVal > 0 {
						if name == "" {
							name = fmt.Sprintf("%d-%s", portVal, strings.ToLower(protocol))
							if protocol == "" {
								name = fmt.Sprintf("%d-tcp", portVal)
							}
						}
						depParams.Service.Ports[name] = struct {
							Port     int    `json:"port" yaml:"port"`
							Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
						}{Port: portVal, Protocol: protocol}
						
						// Maintain backward compatibility for single-port fields
						if depParams.Service.Port == nil || name == "http" {
							pCopy := portVal
							depParams.Service.Port = &pCopy
						}
					}
				}
			}
			params.Deployments[compName] = depParams

		case "ConfigMap":
			data, found, err := unstructured.NestedMap(obj.Object, "data")
			if err == nil && found {
				objName := obj.GetName()
				isMounted := false
				for depName, mappings := range volMappings {
					for _, m := range mappings {
						if m.SourceType == "configMap" && m.SourceName == objName {
							isMounted = true
							depParams := params.Deployments[depName]
							
							var targetFiles map[string]CustomFileParams
							if m.SidecarName != "" {
								if sidecar, ok := depParams.ExtraContainers[m.SidecarName]; ok {
									if sidecar.Config == nil {
										sidecar.Config = &ConfigParams{}
									}
									if sidecar.Config.Files == nil {
										sidecar.Config.Files = make(map[string]CustomFileParams)
									}
									targetFiles = sidecar.Config.Files
								} else if sidecar, ok := depParams.InitContainers[m.SidecarName]; ok {
									if sidecar.Config == nil {
										sidecar.Config = &ConfigParams{}
									}
									if sidecar.Config.Files == nil {
										sidecar.Config.Files = make(map[string]CustomFileParams)
									}
									targetFiles = sidecar.Config.Files
								}
							}
							
							if targetFiles == nil {
								if depParams.Config.Files == nil {
									depParams.Config.Files = make(map[string]CustomFileParams)
								}
								targetFiles = depParams.Config.Files
							}

							for k, v := range data {
								if m.SubPath == "" || m.SubPath == k {
									mntPath := m.MountPath
									if m.SubPath == "" {
										mntPath = filepath.Join(m.MountPath, k)
									}
									targetFiles[k] = CustomFileParams{
										MountPath: mntPath,
										Content:   cleanMultilineString(fmt.Sprintf("%v", v)),
									}
								}
							}
							params.Deployments[depName] = depParams
						}
					}
				}

				if !isMounted {
					if target, ok := envTracker[objName]; ok && !target.IsMain && target.RefCount == 1 {
						depParams := params.Deployments[target.CompName]
						if sidecar, ok := depParams.ExtraContainers[target.SidecarName]; ok {
							for k, v := range data {
								sidecar.Config.Env[k] = cleanMultilineString(fmt.Sprintf("%v", v))
							}
							params.Deployments[target.CompName] = depParams
						}
					} else if compName != "" {
						depParams := params.Deployments[compName]
						for k, v := range data {
							depParams.Config.Env[k] = cleanMultilineString(fmt.Sprintf("%v", v))
						}
						params.Deployments[compName] = depParams
					} else {
						for k, v := range data {
							params.GlobalConfig[k] = cleanMultilineString(fmt.Sprintf("%v", v))
						}
					}
				}
			}

		case "Secret":
			stringData, foundStr, _ := unstructured.NestedMap(obj.Object, "stringData")
			data, foundData, _ := unstructured.NestedMap(obj.Object, "data")

			objName := obj.GetName()
			compName := findComponent(objName, obj.GetLabels())

			isMounted := false
			for depName, mappings := range volMappings {
				for _, m := range mappings {
					if m.SourceType == "secret" && m.SourceName == objName {
						isMounted = true
						depParams := params.Deployments[depName]
						
						var targetFiles map[string]CustomFileParams
						if m.SidecarName != "" {
							if sidecar, ok := depParams.ExtraContainers[m.SidecarName]; ok {
								if sidecar.Secrets == nil {
									sidecar.Secrets = &ConfigParams{}
								}
								if sidecar.Secrets.Files == nil {
									sidecar.Secrets.Files = make(map[string]CustomFileParams)
								}
								targetFiles = sidecar.Secrets.Files
							} else if sidecar, ok := depParams.InitContainers[m.SidecarName]; ok {
								if sidecar.Secrets == nil {
									sidecar.Secrets = &ConfigParams{}
								}
								if sidecar.Secrets.Files == nil {
									sidecar.Secrets.Files = make(map[string]CustomFileParams)
								}
								targetFiles = sidecar.Secrets.Files
							}
						}
						
						if targetFiles == nil {
							if depParams.Secrets.Files == nil {
								depParams.Secrets.Files = make(map[string]CustomFileParams)
							}
							targetFiles = depParams.Secrets.Files
						}
						
						processSecretData := func(sourceData map[string]interface{}, decode bool) {
							for k, v := range sourceData {
								if m.SubPath == "" || m.SubPath == k {
									mntPath := m.MountPath
									if m.SubPath == "" {
										mntPath = filepath.Join(m.MountPath, k)
									}
									contentStr := fmt.Sprintf("%v", v)
									if decode {
										if decoded, err := base64.StdEncoding.DecodeString(contentStr); err == nil {
											contentStr = string(decoded)
										}
									}
									targetFiles[k] = CustomFileParams{
										MountPath: mntPath,
										Content:   cleanMultilineString(contentStr),
									}
								}
							}
						}
						
						if foundStr {
							processSecretData(stringData, false)
						}
						if foundData {
							processSecretData(data, true)
						}
						
						params.Deployments[depName] = depParams
					}
				}
			}

			if !isMounted {
				if target, ok := envTracker[objName]; ok && !target.IsMain && target.RefCount == 1 {
					depParams := params.Deployments[target.CompName]
					if sidecar, ok := depParams.ExtraContainers[target.SidecarName]; ok {
						if foundStr {
							for k, v := range stringData {
								sidecar.Secrets.Env[k] = cleanMultilineString(fmt.Sprintf("%v", v))
							}
						}
						if foundData {
							for k, v := range data {
								if strVal, ok := v.(string); ok {
									if decoded, err := base64.StdEncoding.DecodeString(strVal); err == nil {
										sidecar.Secrets.Env[k] = cleanMultilineString(string(decoded))
									} else {
										sidecar.Secrets.Env[k] = cleanMultilineString(strVal)
									}
								}
							}
						}
						params.Deployments[target.CompName] = depParams
					}
				} else if compName == "" {
					if foundStr {
						for k, v := range stringData {
							params.GlobalSecret[k] = cleanMultilineString(fmt.Sprintf("%v", v))
						}
					}
					if foundData {
						for k, v := range data {
							if strVal, ok := v.(string); ok {
								if decoded, err := base64.StdEncoding.DecodeString(strVal); err == nil {
									params.GlobalSecret[k] = cleanMultilineString(string(decoded))
								} else {
									params.GlobalSecret[k] = cleanMultilineString(strVal)
								}
							}
						}
					}
				} else {
					depParams := params.Deployments[compName]
					if foundStr {
						for k, v := range stringData {
							depParams.Secrets.Env[k] = cleanMultilineString(fmt.Sprintf("%v", v))
						}
					}
					if foundData {
						for k, v := range data {
							if strVal, ok := v.(string); ok {
								if decoded, err := base64.StdEncoding.DecodeString(strVal); err == nil {
									depParams.Secrets.Env[k] = cleanMultilineString(string(decoded))
								} else {
									depParams.Secrets.Env[k] = cleanMultilineString(strVal)
								}
							}
						}
					}
					params.Deployments[compName] = depParams
				}
			}

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
			
			hpaParams := &HpaParams{
				Enabled: true,
			}
			if min, ok, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicas"); ok {
				hpaParams.MinReplicas = int(min)
			}
			if max, ok, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicas"); ok {
				hpaParams.MaxReplicas = int(max)
			}
			if metrics, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "metrics"); ok && len(metrics) > 0 {
				hpaParams.Metrics = metrics
			}
			if behavior, ok, _ := unstructured.NestedMap(obj.Object, "spec", "behavior"); ok && len(behavior) > 0 {
				hpaParams.Behavior = behavior
			}
			if depParams.Autoscaling == nil {
				depParams.Autoscaling = &AutoscalingParams{}
			}
			depParams.Autoscaling.Enabled = true
			depParams.Autoscaling.Engine = "hpa"
			depParams.Autoscaling.Hpa = hpaParams
			params.Deployments[compName] = depParams
		}
	}


	// Intercept CronJob components and map them to subcomponents
	for compName, depParams := range params.Deployments {
		if depParams.WorkloadType == "CronJob" {
			foundSub := false
			for _, sub := range params.Subcomponents {
				if sub == "cronjob" {
					foundSub = true
					break
				}
			}
			if !foundSub {
				params.Subcomponents = append(params.Subcomponents, "cronjob")
			}
			
			if params.SubcomponentsData == nil {
				params.SubcomponentsData = make(map[string]interface{})
			}
			
			cronjobData := map[string]interface{}{
				"enabled": true,
				"schedule": depParams.Schedule,
				"image": map[string]interface{}{
					"repository": depParams.Image.Repository,
					"tag": depParams.Image.Tag,
					"pullPolicy": "IfNotPresent",
				},
				"concurrencyPolicy": "Allow",
				"successfulJobsHistoryLimit": 3,
				"failedJobsHistoryLimit": 1,
				"restartPolicy": "OnFailure",
			}
			
			if len(depParams.Command) > 0 {
				cronjobData["command"] = depParams.Command
			}
			if len(depParams.Args) > 0 {
				cronjobData["args"] = depParams.Args
			}
			if depParams.Config != nil && len(depParams.Config.Env) > 0 {
				cronjobData["config"] = map[string]interface{}{"env": depParams.Config.Env}
			}
			if depParams.Secrets != nil && len(depParams.Secrets.Env) > 0 {
				cronjobData["secrets"] = map[string]interface{}{"env": depParams.Secrets.Env}
			}
			if depParams.Resources != nil {
				cronjobData["resources"] = depParams.Resources
			}
			if depParams.Scheduling != nil {
				cronjobData["scheduling"] = depParams.Scheduling
			}
			if depParams.Persistence.Enabled {
				cronjobData["persistence"] = map[string]interface{}{
					"enabled": true,
					"mountPath": depParams.Persistence.MountPath,
					"size": "1Gi",
					"accessMode": "ReadWriteOnce",
				}
			}
			// Map configMap and Secret files
			if (depParams.Config != nil && len(depParams.Config.Files) > 0) || (depParams.Secrets != nil && len(depParams.Secrets.Files) > 0) {
				if depParams.Config != nil && len(depParams.Config.Files) > 0 {
					cmMap := make(map[string]interface{})
					for k, v := range depParams.Config.Files {
						cmMap[k] = map[string]interface{}{
							"mountPath": v.MountPath,
							"content": v.Content,
						}
					}
					if configMap, ok := cronjobData["config"].(map[string]interface{}); ok {
						configMap["files"] = cmMap
					} else {
						cronjobData["config"] = map[string]interface{}{"files": cmMap}
					}
				}
				if depParams.Secrets != nil && len(depParams.Secrets.Files) > 0 {
					secMap := make(map[string]interface{})
					for k, v := range depParams.Secrets.Files {
						secMap[k] = map[string]interface{}{
							"mountPath": v.MountPath,
							"content": v.Content,
						}
					}
					if secData, ok := cronjobData["secrets"].(map[string]interface{}); ok {
						secData["files"] = secMap
					} else {
						cronjobData["secrets"] = map[string]interface{}{"files": secMap}
					}
				}
			}

			// Assign to SubcomponentsData and remove from Deployments
			params.SubcomponentsData[compName] = cronjobData
			delete(params.Deployments, compName)
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

	logrus.Info("Pass 5 complete")
	return params, nil
}


func populateContainerParams(depParams *DeploymentParams, container map[string]interface{}, volSources map[string]struct{Type string; Name string}, envTracker map[string]*EnvTarget, compName string) {
	image, _, _ := unstructured.NestedString(container, "image")
	if image != "" {
		index := strings.LastIndex(image, ":")
		if strings.Contains(image, "@") && strings.Count(image, ":") >= 2 {
			last := strings.LastIndex(image, ":")
			index = strings.LastIndex(image[:last], ":")
		}
		if index < 0 {
			depParams.Image.Repository = image
			depParams.Image.Tag = "latest"
		} else {
			depParams.Image.Repository = image[:index]
			depParams.Image.Tag = image[index+1:]
		}
	}
	if command, found, _ := unstructured.NestedStringSlice(container, "command"); found && len(command) > 0 {
		depParams.Command = command
	}

	if args, found, _ := unstructured.NestedStringSlice(container, "args"); found && len(args) > 0 {
		depParams.Args = args
	}

	if ports, found, _ := unstructured.NestedSlice(container, "ports"); found && len(ports) > 0 {
		if depParams.Service.Ports == nil {
			depParams.Service.Ports = make(map[string]struct{Port int `json:"port" yaml:"port"`; Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`})
		}
		for _, p := range ports {
			portMap := p.(map[string]interface{})
			name, _, _ := unstructured.NestedString(portMap, "name")
			containerPort, _, _ := unstructured.NestedInt64(portMap, "containerPort")
			
			if name == "" {
				name = fmt.Sprintf("%d-tcp", containerPort)
			}
			depParams.Service.Ports[name] = struct{Port int `json:"port" yaml:"port"`; Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`}{
				Port:     int(containerPort),
			}
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
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Startup = &probe
			}
		}
	}
	if p, ok, _ := unstructured.NestedMap(container, "livenessProbe"); ok && len(p) > 0 {
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Liveness = &probe
			}
		}
	}
	if p, ok, _ := unstructured.NestedMap(container, "readinessProbe"); ok && len(p) > 0 {
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Readiness = &probe
			}
		}
	}

	// Extract Persistence from volumeMounts
	mounts, ok, _ := unstructured.NestedSlice(container, "volumeMounts")
	if ok && len(mounts) > 0 {
		for _, m := range mounts {
			mount := m.(map[string]interface{})
			name, _, _ := unstructured.NestedString(mount, "name")

			_, isConfigMapOrSecret := volSources[name]
			if !strings.HasPrefix(name, "kube-api") && !strings.Contains(name, "default-token") && !isConfigMapOrSecret {
				path, _, _ := unstructured.NestedString(mount, "mountPath")
				depParams.Persistence.Enabled = true
				depParams.Persistence.MountPath = path
				break // Take the first meaningful volume for the simple model
			}
		}
	}

	// Extract envFrom for main container
	envFrom, ok, _ := unstructured.NestedSlice(container, "envFrom")
	if ok && len(envFrom) > 0 {
		for _, e := range envFrom {
			if envMap, ok := e.(map[string]interface{}); ok {
				if cmRef, ok, _ := unstructured.NestedMap(envMap, "configMapRef"); ok {
					if cmName, _, _ := unstructured.NestedString(cmRef, "name"); cmName != "" {
						if target, exists := envTracker[cmName]; exists {
							target.IsMain = true
						} else {
							envTracker[cmName] = &EnvTarget{IsMain: true}
						}
					}
				}
				if secRef, ok, _ := unstructured.NestedMap(envMap, "secretRef"); ok {
					if secName, _, _ := unstructured.NestedString(secRef, "name"); secName != "" {
						if target, exists := envTracker[secName]; exists {
							target.IsMain = true
						} else {
							envTracker[secName] = &EnvTarget{IsMain: true}
						}
					}
				}
			}
		}
	}
}


func populateSidecarParams(depParams *SidecarParams, container map[string]interface{}, volSources map[string]struct{Type string; Name string}, envTracker map[string]*EnvTarget, compName string, sidecarName string) {
	image, _, _ := unstructured.NestedString(container, "image")
	if image != "" {
		index := strings.LastIndex(image, ":")
		if strings.Contains(image, "@") && strings.Count(image, ":") >= 2 {
			last := strings.LastIndex(image, ":")
			index = strings.LastIndex(image[:last], ":")
		}
		if index < 0 {
			depParams.Image.Repository = image
			depParams.Image.Tag = "latest"
		} else {
			depParams.Image.Repository = image[:index]
			depParams.Image.Tag = image[index+1:]
		}
	}
	if command, found, _ := unstructured.NestedStringSlice(container, "command"); found && len(command) > 0 {
		depParams.Command = command
	}

	if args, found, _ := unstructured.NestedStringSlice(container, "args"); found && len(args) > 0 {
		depParams.Args = args
	}

	if ports, found, _ := unstructured.NestedSlice(container, "ports"); found && len(ports) > 0 {
		if depParams.Service.Ports == nil {
			depParams.Service.Ports = make(map[string]struct{Port int `json:"port" yaml:"port"`; Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`})
		}
		for _, p := range ports {
			portMap := p.(map[string]interface{})
			name, _, _ := unstructured.NestedString(portMap, "name")
			containerPort, _, _ := unstructured.NestedInt64(portMap, "containerPort")
			
			if name == "" {
				name = fmt.Sprintf("%d-tcp", containerPort)
			}
			depParams.Service.Ports[name] = struct{Port int `json:"port" yaml:"port"`; Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`}{
				Port:     int(containerPort),
			}
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
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Startup = &probe
			}
		}
	}
	if p, ok, _ := unstructured.NestedMap(container, "livenessProbe"); ok && len(p) > 0 {
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Liveness = &probe
			}
		}
	}
	if p, ok, _ := unstructured.NestedMap(container, "readinessProbe"); ok && len(p) > 0 {
		if b, err := json.Marshal(p); err == nil {
			var probe ProbeParams
			if err := json.Unmarshal(b, &probe); err == nil {
				if depParams.Probes == nil {
					depParams.Probes = &ProbesParams{}
				}
				depParams.Probes.Readiness = &probe
			}
		}
	}

	// Extract Persistence from volumeMounts
	mounts, ok, _ := unstructured.NestedSlice(container, "volumeMounts")
	if ok && len(mounts) > 0 {
		for _, m := range mounts {
			mount := m.(map[string]interface{})
			name, _, _ := unstructured.NestedString(mount, "name")

			_, isConfigMapOrSecret := volSources[name]
			if !strings.HasPrefix(name, "kube-api") && !strings.Contains(name, "default-token") && !isConfigMapOrSecret {
				path, _, _ := unstructured.NestedString(mount, "mountPath")
				depParams.Persistence.Enabled = true
				depParams.Persistence.MountPath = path
				break // Take the first meaningful volume for the simple model
			}
		}
	}

	// Extract envFrom for sidecar
	envFrom, ok, _ := unstructured.NestedSlice(container, "envFrom")
	if ok && len(envFrom) > 0 {
		for _, e := range envFrom {
			if envMap, ok := e.(map[string]interface{}); ok {
				if cmRef, ok, _ := unstructured.NestedMap(envMap, "configMapRef"); ok {
					if cmName, _, _ := unstructured.NestedString(cmRef, "name"); cmName != "" {
						if target, exists := envTracker[cmName]; exists {
							target.RefCount++
						} else {
							envTracker[cmName] = &EnvTarget{CompName: compName, SidecarName: sidecarName, RefCount: 1}
						}
					}
				}
				if secRef, ok, _ := unstructured.NestedMap(envMap, "secretRef"); ok {
					if secName, _, _ := unstructured.NestedString(secRef, "name"); secName != "" {
						if target, exists := envTracker[secName]; exists {
							target.RefCount++
						} else {
							envTracker[secName] = &EnvTarget{CompName: compName, SidecarName: sidecarName, RefCount: 1}
						}
					}
				}
			}
		}
	}
}


