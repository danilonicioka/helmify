package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleDownload_ValuesEnvOutsideChart(t *testing.T) {
	reqBody := downloadRequest{
		ChartName: "test-app",
		Files: map[string]string{
			"Chart.yaml":       "apiVersion: v2\nname: test-app\nversion: 0.1.0",
			"values.yaml":      "test-app:\n  replicas: 1",
			"values-env.yaml":  "global:\n  TZ: America/Belem",
			".gitlab-ci.yml":   "stages: [deploy]",
			"README.md":        "# Test App",
			"templates/d.yaml": "kind: Deployment",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/download", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleDownload(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/x-tar", resp.Header.Get("Content-Type"))

	// Read and verify tar.gz contents
	gr, err := gzip.NewReader(resp.Body)
	assert.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	paths := make(map[string]bool)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		paths[hdr.Name] = true
	}

	// Verify operational overlay, README, and CI are at root outside the chart folder
	assert.True(t, paths["values-env.yaml"], "values-env.yaml should be outside chart directory")
	assert.False(t, paths["chart/values-env.yaml"], "values-env.yaml must NOT be inside chart directory")
	assert.True(t, paths["README.md"], "README.md should be at root")
	assert.True(t, paths[".gitlab-ci.yml"], ".gitlab-ci.yml should be at root")

	// Verify chart files are inside chart/
	assert.True(t, paths["chart/Chart.yaml"], "Chart.yaml should be inside chart/")
	assert.True(t, paths["chart/values.yaml"], "values.yaml should be inside chart/")
}

func TestHandleGenerate_RouteAndVolumes(t *testing.T) {
	manifests := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  replicas: 2
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: quay.io/myorg/myapp:1.0.0
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: my-storage
          mountPath: /data
      volumes:
      - name: my-storage
        persistentVolumeClaim:
          claimName: my-pvc
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  labels:
    app: myapp
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  host: myapp.example.com
  path: /api
  to:
    kind: Service
    name: myapp
`

	req := httptest.NewRequest(http.MethodPost, "/v1/generate", bytes.NewBufferString(manifests))
	req.Header.Set("X-Chart-Name", "myapp")
	w := httptest.NewRecorder()

	handleGenerate(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	gr, err := gzip.NewReader(resp.Body)
	assert.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, tr)
		files[hdr.Name] = buf.String()
	}

	valuesEnv := files["values-env.yaml"]
	assert.Contains(t, valuesEnv, "routes:")
	assert.Contains(t, valuesEnv, "host: myapp.example.com")
	assert.Contains(t, valuesEnv, "path: /api")
	assert.NotContains(t, valuesEnv, "\n    route:\n")

	assert.Contains(t, valuesEnv, "volumes:")
	assert.Contains(t, valuesEnv, "my-storage:")
	assert.Contains(t, valuesEnv, "mountPath: /data")
	assert.Contains(t, valuesEnv, "storageRequest: 5Gi")
	assert.Contains(t, valuesEnv, "accessMode: ReadWriteOnce")
	assert.NotContains(t, valuesEnv, "\n    persistence:\n")
}

func TestHandleGenerate_MultiRouteAndMultiVolumes(t *testing.T) {
	manifests := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: multiapp
  labels:
    app: multiapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: multiapp
  template:
    metadata:
      labels:
        app: multiapp
    spec:
      containers:
      - name: multiapp
        image: quay.io/myorg/multiapp:2.0.0
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: data-vol
          mountPath: /var/data
        - name: tmp-cache
          mountPath: /tmp/cache
      volumes:
      - name: data-vol
        persistentVolumeClaim:
          claimName: multiapp-pvc
      - name: tmp-cache
        emptyDir: {}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: multiapp-pvc
  labels:
    app: multiapp
spec:
  storageClassName: ocs-storagecluster-ceph-rbd
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: multiapp
  labels:
    app: multiapp
spec:
  host: multiapp.apps.example.com
  path: /
  to:
    kind: Service
    name: multiapp
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: multiapp-metrics
  labels:
    app: multiapp
spec:
  host: multiapp-metrics.apps.example.com
  path: /metrics
  to:
    kind: Service
    name: multiapp
`

	req := httptest.NewRequest(http.MethodPost, "/v1/generate", bytes.NewBufferString(manifests))
	req.Header.Set("X-Chart-Name", "multiapp")
	w := httptest.NewRecorder()

	handleGenerate(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	gr, err := gzip.NewReader(resp.Body)
	assert.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, tr)
		files[hdr.Name] = buf.String()
	}

	valuesEnv := files["values-env.yaml"]

	// Verify routes mapping
	assert.Contains(t, valuesEnv, "routes:")
	assert.Contains(t, valuesEnv, "host: multiapp.apps.example.com")
	assert.Contains(t, valuesEnv, "host: multiapp-metrics.apps.example.com")
	assert.Contains(t, valuesEnv, "path: /metrics")

	// Verify volumes mapping: data-vol (PVC) and tmp-cache (emptyDir)
	assert.Contains(t, valuesEnv, "volumes:")
	assert.Contains(t, valuesEnv, "data-vol:")
	assert.Contains(t, valuesEnv, "mountPath: /var/data")
	assert.Contains(t, valuesEnv, "storageRequest: 20Gi")
	assert.Contains(t, valuesEnv, "accessMode: ReadWriteOnce")
	assert.Contains(t, valuesEnv, "storageClass: ocs-storagecluster-ceph-rbd")

	assert.Contains(t, valuesEnv, "tmp-cache:")
	assert.Contains(t, valuesEnv, "mountPath: /tmp/cache")
	assert.Contains(t, valuesEnv, "ephemeral: true")

	// Validate actual Helm rendering of the output files
	tempDir := t.TempDir()
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		assert.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	chartPath := filepath.Join(tempDir, "chart")
	valuesEnvPath := filepath.Join(tempDir, "values-env.yaml")
	cmd := exec.Command("helm", "template", "multiapp", chartPath, "-f", valuesEnvPath)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template failed: %s", string(out))

	rendered := string(out)
	// Routes verified in rendered manifests
	assert.Contains(t, rendered, "host: \"multiapp.apps.example.com\"")
	assert.Contains(t, rendered, "host: \"multiapp-metrics.apps.example.com\"")
	assert.Contains(t, rendered, "path: \"/metrics\"")

	// Volumes & PVC verified in rendered manifests
	assert.Contains(t, rendered, "claimName: multiapp-data-vol")
	assert.Contains(t, rendered, "mountPath: /var/data")
	assert.Contains(t, rendered, "mountPath: /tmp/cache")
	assert.Contains(t, rendered, "storage: 20Gi")
	assert.Contains(t, rendered, "storageClassName: \"ocs-storagecluster-ceph-rbd\"")
}


