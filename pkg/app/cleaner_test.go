package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/arttor/helmify/pkg/config"
	"github.com/arttor/helmify/pkg/helm"
	"github.com/arttor/helmify/pkg/translator/k8smanifest"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSanitizeObject(t *testing.T) {
	t.Run("Discards Namespace Kind", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": "my-namespace",
				},
			},
		}
		keep := sanitizeObject(obj)
		assert.False(t, keep)
	})

	t.Run("Keeps other kinds and normalizes ports", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "my-deploy",
					"namespace": "my-namespace",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "app",
									"ports": []interface{}{
										map[string]interface{}{
											"name":          "3000-tcp",
											"containerPort": int64(3000),
										},
										map[string]interface{}{
											"name":          "udp-80",
											"containerPort": int64(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}

		keep := sanitizeObject(obj)
		assert.True(t, keep)

		// Namespace metadata should NOT be deleted prior to appMeta loading
		metadata := obj.Object["metadata"].(map[string]interface{})
		assert.Equal(t, "my-namespace", metadata["namespace"])

		// Port names should be normalized (e.g. 3000-tcp -> tcp-3000)
		spec := obj.Object["spec"].(map[string]interface{})
		template := spec["template"].(map[string]interface{})
		podSpec := template["spec"].(map[string]interface{})
		containers := podSpec["containers"].([]interface{})
		c := containers[0].(map[string]interface{})
		ports := c["ports"].([]interface{})

		port1 := ports[0].(map[string]interface{})
		assert.Equal(t, "tcp-3000", port1["name"])

		port2 := ports[1].(map[string]interface{})
		assert.Equal(t, "udp-80", port2["name"]) // unchanged since it's already in format protocol-port
	})
}

const userManifestYaml = `apiVersion: v1
data:
  JURISPRUDENCIA_AGENT_LLM: azure
  TZ: America/Sao_Paulo
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/instance: jurisprudencia-dev
  name: jurisprudencia-dev-g88f7cc858
  namespace: tjpa-iande
---
apiVersion: v1
data:
  JURISPRUDENCIA_QDRANT_API_KEY: YWdEeFF3WXpwSVYxdyNWbmY=
kind: Secret
metadata:
  labels:
    app.kubernetes.io/instance: jurisprudencia-dev
  name: jurisprudencia-dev-5tt65kb6bk
  namespace: tjpa-iande
type: Opaque
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: app
  name: app-dev
  namespace: tjpa-iande
spec:
  ports:
  - name: http
    port: 22650
    targetPort: 22650
  selector:
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: app
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: app
  name: app-dev
  namespace: tjpa-iande
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/instance: jurisprudencia-dev
      app.kubernetes.io/name: app
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: jurisprudencia-dev
        app.kubernetes.io/name: app
    spec:
      containers:
      - envFrom:
        - secretRef:
            name: jurisprudencia-dev-5tt65kb6bk
        - configMapRef:
            name: jurisprudencia-dev-g88f7cc858
        image: quay.io/ca/ia/jurisprudencia:main-3fb7da28
        imagePullPolicy: Always
        name: app
        ports:
        - containerPort: 22650
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component: worker
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: celery
  name: celery-dev
  namespace: tjpa-iande
spec:
  replicas: 20
  selector:
    matchLabels:
      app.kubernetes.io/instance: jurisprudencia-dev
      app.kubernetes.io/name: celery
  template:
    metadata:
      labels:
        app.kubernetes.io/component: worker
        app.kubernetes.io/instance: jurisprudencia-dev
        app.kubernetes.io/name: celery
    spec:
      containers:
      - command:
        - uv
        - run
        - celery
        - worker
        envFrom:
        - secretRef:
            name: jurisprudencia-dev-5tt65kb6bk
        - configMapRef:
            name: jurisprudencia-dev-g88f7cc858
        image: quay.io/ca/ia/jurisprudencia:main-3fb7da28
        imagePullPolicy: Always
        name: celery
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component: listener
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: kombu
  name: kombu-dev
  namespace: tjpa-iande
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/instance: jurisprudencia-dev
      app.kubernetes.io/name: kombu
  template:
    metadata:
      labels:
        app.kubernetes.io/component: listener
        app.kubernetes.io/instance: jurisprudencia-dev
        app.kubernetes.io/name: kombu
    spec:
      containers:
      - command:
        - uv
        - run
        - python
        envFrom:
        - secretRef:
            name: jurisprudencia-dev-5tt65kb6bk
        - configMapRef:
            name: jurisprudencia-dev-g88f7cc858
        image: quay.io/ca/ia/jurisprudencia:main-3fb7da28
        imagePullPolicy: Always
        name: kombu
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  labels:
    app.kubernetes.io/instance: jurisprudencia-dev
    app.kubernetes.io/name: app
  name: jurisprudencia-app-dev
  namespace: tjpa-iande
spec:
  host: iande.apps.ocp-dev.i.tj.pa.gov.br
  port:
    targetPort: http
  tls:
    insecureEdgeTerminationPolicy: Redirect
    termination: edge
  to:
    kind: Service
    name: app-dev
    weight: 100`

func TestUserManifestConversion(t *testing.T) {
	reader := strings.NewReader(userManifestYaml)
	conf := config.Config{ChartName: "jurisprudencia"}
	
	// We'll test decoding and running the engine to verify lint and naming passes.
	trans := k8smanifest.New(conf, reader)
	engine := NewEngine(conf, helm.NewOutput())

	err := engine.Run(context.Background(), trans)
	assert.NoError(t, err)

	defer os.RemoveAll("jurisprudencia")

	valuesBytes, err := os.ReadFile("jurisprudencia/values.yaml")
	assert.NoError(t, err)
	valuesStr := string(valuesBytes)

	assert.Contains(t, valuesStr, "global:")
	assert.Contains(t, valuesStr, "  cm:")
	assert.Contains(t, valuesStr, "    JURISPRUDENCIA_AGENT_LLM: azure")
	assert.Contains(t, valuesStr, "  secret:")
	assert.Contains(t, valuesStr, "    JURISPRUDENCIA_QDRANT_API_KEY: agDxQwYzpIV1w#Vnf")
}

