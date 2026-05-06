package pod

import (
	"testing"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/metadata"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/arttor/helmify/internal"
	"github.com/stretchr/testify/assert"
)

const (
	strDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        args:
        - --test
        - --arg
        ports:
        - containerPort: 80
`

	strDeploymentWithTagAndDigest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2@sha256:cb5c1bddd1b5665e1867a7fa1b5fa843a47ee433bbb75d4293888b71def53229
        ports:
        - containerPort: 80
`

	strDeploymentWithNoArgs = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`

	strDeploymentWithPort = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: localhost:6001/my_project:latest
        ports:
        - containerPort: 80
`
	strDeploymentWithPodSecurityContext = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: localhost:6001/my_project:latest
      securityContext:
        fsGroup: 20000
        runAsGroup: 30000
        runAsNonRoot: true
        runAsUser: 65532

`
)

func Test_pod_Process(t *testing.T) {
	t.Run("deployment with args", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeployment)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec, 0)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"args": "{{- toYaml .Values.nginx.args | nindent 8 }}",
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{
								"name": `{{ include ".fullname" . }}-global`,
							},
						},
					},
					"image": "{{ .Values.nginx.image.repository }}:{{ .Values.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources":      "[HELMIFY_WITH:nginx.resources:10]",
					"livenessProbe":  "[HELMIFY_WITH:nginx.livenessProbe:10]",
					"readinessProbe": "[HELMIFY_WITH:nginx.readinessProbe:10]",
					"startupProbe":   "[HELMIFY_WITH:nginx.startupProbe:10]",
				},
			},
			"tolerations":               "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
			"topologySpreadConstraints": "{{- toYaml .Values.nginx.topologySpreadConstraints | nindent 8 }}",
			"nodeSelector":              "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
			"serviceAccountName":        `{{ include ".serviceAccountName" . }}`,
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": "nginx",
					"tag":        "1.14.2",
				},
				"args": []interface{}{
					"--test",
					"--arg",
				},
				"livenessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"readinessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"startupProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"nodeSelector":              map[string]interface{}{},
				"tolerations":               []interface{}{},
				"topologySpreadConstraints": []interface{}{},
			},
		}, tmpl)
	})

	t.Run("deployment with no args", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithNoArgs)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec, 0)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{
								"name": `{{ include ".fullname" . }}-global`,
							},
						},
					},
					"image": "{{ .Values.nginx.image.repository }}:{{ .Values.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources":      "[HELMIFY_WITH:nginx.resources:10]",
					"livenessProbe":  "[HELMIFY_WITH:nginx.livenessProbe:10]",
					"readinessProbe": "[HELMIFY_WITH:nginx.readinessProbe:10]",
					"startupProbe":   "[HELMIFY_WITH:nginx.startupProbe:10]",
				},
			},
			"nodeSelector":              "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
			"serviceAccountName":        `{{ include ".serviceAccountName" . }}`,
			"tolerations":               "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
			"topologySpreadConstraints": "{{- toYaml .Values.nginx.topologySpreadConstraints | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": "nginx",
					"tag":        "1.14.2",
				},
				"livenessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"readinessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"startupProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"nodeSelector":              map[string]interface{}{},
				"tolerations":               []interface{}{},
				"topologySpreadConstraints": []interface{}{},
			},
		}, tmpl)
	})

	t.Run("deployment with image tag and digest", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithTagAndDigest)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec, 0)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{
								"name": `{{ include ".fullname" . }}-global`,
							},
						},
					},
					"image": "{{ .Values.nginx.image.repository }}:{{ .Values.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources":      "[HELMIFY_WITH:nginx.resources:10]",
					"livenessProbe":  "[HELMIFY_WITH:nginx.livenessProbe:10]",
					"readinessProbe": "[HELMIFY_WITH:nginx.readinessProbe:10]",
					"startupProbe":   "[HELMIFY_WITH:nginx.startupProbe:10]",
				},
			},
			"nodeSelector":              "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
			"serviceAccountName":        `{{ include ".serviceAccountName" . }}`,
			"tolerations":               "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
			"topologySpreadConstraints": "{{- toYaml .Values.nginx.topologySpreadConstraints | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": "nginx",
					"tag":        "1.14.2@sha256:cb5c1bddd1b5665e1867a7fa1b5fa843a47ee433bbb75d4293888b71def53229",
				},
				"livenessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"readinessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"startupProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"nodeSelector":              map[string]interface{}{},
				"tolerations":               []interface{}{},
				"topologySpreadConstraints": []interface{}{},
			},
		}, tmpl)
	})

	t.Run("deployment with image tag and port", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithPort)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec, 0)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{
								"name": `{{ include ".fullname" . }}-global`,
							},
						},
					},
					"image": "{{ .Values.nginx.image.repository }}:{{ .Values.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources":      "[HELMIFY_WITH:nginx.resources:10]",
					"livenessProbe":  "[HELMIFY_WITH:nginx.livenessProbe:10]",
					"readinessProbe": "[HELMIFY_WITH:nginx.readinessProbe:10]",
					"startupProbe":   "[HELMIFY_WITH:nginx.startupProbe:10]",
				},
			},
			"nodeSelector":              "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
			"serviceAccountName":        `{{ include ".serviceAccountName" . }}`,
			"tolerations":               "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
			"topologySpreadConstraints": "{{- toYaml .Values.nginx.topologySpreadConstraints | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": "localhost:6001/my_project",
					"tag":        "latest",
				},
				"livenessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"readinessProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"startupProbe": map[string]interface{}{
					"initialDelaySeconds": int64(0),
					"periodSeconds":       int64(10),
					"tcpSocket": map[string]interface{}{
						"port": int64(80),
					},
				},
				"nodeSelector":              map[string]interface{}{},
				"tolerations":               []interface{}{},
				"topologySpreadConstraints": []interface{}{},
			},
		}, tmpl)
	})
	t.Run("deployment with securityContext", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithPodSecurityContext)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec, 0)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{
								"name": `{{ include ".fullname" . }}-global`,
							},
						},
					},
					"image":          "{{ .Values.nginx.image.repository }}:{{ .Values.nginx.image.tag | default .Chart.AppVersion }}",
					"name":           "nginx",
					"resources":      "[HELMIFY_WITH:nginx.resources:10]",
					"livenessProbe":  "[HELMIFY_WITH:nginx.livenessProbe:10]",
					"readinessProbe": "[HELMIFY_WITH:nginx.readinessProbe:10]",
					"startupProbe":   "[HELMIFY_WITH:nginx.startupProbe:10]",
				},
			},
			"securityContext":           "{{- toYaml .Values.nginx.podSecurityContext | nindent 8 }}",
			"nodeSelector":              "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
			"serviceAccountName":        `{{ include ".serviceAccountName" . }}`,
			"tolerations":               "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
			"topologySpreadConstraints": "{{- toYaml .Values.nginx.topologySpreadConstraints | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"podSecurityContext": map[string]interface{}{
					"fsGroup":      int64(20000),
					"runAsGroup":   int64(30000),
					"runAsNonRoot": true,
					"runAsUser":    int64(65532),
				},
				"image": map[string]interface{}{
					"repository": "localhost:6001/my_project",
					"tag":        "latest",
				},
				"livenessProbe":             map[string]interface{}{},
				"readinessProbe":            map[string]interface{}{},
				"startupProbe":              map[string]interface{}{},
				"nodeSelector":              map[string]interface{}{},
				"tolerations":               []interface{}{},
				"topologySpreadConstraints": []interface{}{},
			},
		}, tmpl)
	})

	t.Run("placeholder replacement", func(t *testing.T) {
		input := `
        livenessProbe: '[HELMIFY_WITH:nginx.livenessProbe:10]'
        resources: '[HELMIFY_WITH:nginx.resources:10]'`
		output := ReplacePlaceholders(input)
		assert.Contains(t, output, "{{- with .Values.nginx.livenessProbe }}")
		assert.Contains(t, output, "livenessProbe:")
		assert.Contains(t, output, "{{- with .Values.nginx.resources }}")
		assert.Contains(t, output, "resources:")
	})

}
