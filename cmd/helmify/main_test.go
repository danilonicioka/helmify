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
	"strings"
	"testing"

	"github.com/danilonicioka/helmify/pkg/helm"
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

func TestRouteFallback_FromDefault(t *testing.T) {
	wizardParams := helm.WizardParams{
		ChartName: "test-route-fallback",
		Deployments: map[string]helm.DeploymentParams{
			"app": {
				Image: helm.ImageParams{Repository: "test/app", Tag: "latest"},
				Routes: map[string]helm.SubRouteParams{
					"default": {
						Enabled: true,
						Host:    "app.default.domain",
						Path:    "/",
					},
					"external": {
						Enabled: true,
						Host:    "app.external.domain",
						// Path, annotations, and TLS omitted, should inherit from default
					},
				},
			},
		},
	}

	files, err := helm.GenerateWizardChart(wizardParams)
	assert.NoError(t, err)

	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	for path, content := range files {
		if path == "values-env.yaml" {
			envStr := string(content)
			envStr = strings.Replace(envStr, "        annotations: {}", "        annotations:\n          haproxy.router.openshift.io/timeout: 45s", 1)
			assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "values-env.yaml"), []byte(envStr), 0644))
			continue
		}
		targetPath := filepath.Join(chartDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, content, 0644))
	}


	chartPath := filepath.Join(tempDir, "chart")
	valuesEnvPath := filepath.Join(tempDir, "values-env.yaml")
	cmd := exec.Command("helm", "template", "test-route-fallback", chartPath, "-f", valuesEnvPath, "-s", "templates/route.yaml")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template failed: %s", string(out))

	rendered := string(out)
	docs := strings.Split(rendered, "---")
	var extDoc string
	for _, doc := range docs {
		if strings.Contains(doc, "app.external.domain") {
			extDoc = doc
			break
		}
	}
	assert.NotEmpty(t, extDoc, "external route must be rendered")
	assert.Contains(t, extDoc, "haproxy.router.openshift.io/timeout: 45s", "annotations must be inherited from default route")
	assert.Contains(t, extDoc, "path: \"/\"", "path must be inherited from default route")
	assert.Contains(t, extDoc, "termination: edge", "tls termination must be inherited from default route")
}

func TestInitAndExtraContainers_VolumesAndSharedVolumes(t *testing.T) {
	wizardParams := helm.WizardParams{
		ChartName: "test-sidecar-vols",
		Deployments: map[string]helm.DeploymentParams{
			"core": {
				Image: helm.ImageParams{Repository: "test/core", Tag: "1.0"},
				Volumes: map[string]helm.VolumeParams{
					"main-vol": {
						Enabled:        true,
						MountPath:      "/var/data",
						StorageRequest: "5Gi",
					},
				},
				InitContainers: map[string]*helm.SidecarParams{
					"setup": {
						Image:   helm.ImageParams{Repository: "test/setup", Tag: "latest"},
						Command: []string{"sh", "-c", "echo init"},
						SharedVolumes: map[string]helm.SharedVolumeParams{
							"main-vol": {
								MountPath: "/shared/data",
							},
						},
						Volumes: map[string]helm.VolumeParams{
							"init-scratch": {
								Enabled:   true,
								Ephemeral: true,
								MountPath: "/scratch",
							},
						},
					},
					"bare-init": {
						// No sharedVolumes or volumes defined - should not cause errors
						Image:   helm.ImageParams{Repository: "test/bare", Tag: "latest"},
						Command: []string{"sh", "-c", "echo ready"},
					},
				},
				ExtraContainers: map[string]*helm.SidecarParams{
					"sidecar": {
						Image: helm.ImageParams{Repository: "test/sidecar", Tag: "latest"},
						SharedVolumes: map[string]helm.SharedVolumeParams{
							"main-vol": {
								MountPath: "/sidecar/shared",
								ReadOnly:  true,
							},
						},
						Volumes: map[string]helm.VolumeParams{
							"sidecar-pvc": {
								Enabled:        true,
								MountPath:      "/sidecar/storage",
								StorageRequest: "2Gi",
							},
						},
					},
					"bare-sidecar": {
						// No sharedVolumes or volumes defined - should not cause errors
						Image: helm.ImageParams{Repository: "test/bare-sidecar", Tag: "latest"},
					},
				},
			},
		},
	}

	files, err := helm.GenerateWizardChart(wizardParams)
	assert.NoError(t, err)

	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	for path, content := range files {
		if path == "values-env.yaml" {
			assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "values-env.yaml"), content, 0644))
			continue
		}
		targetPath := filepath.Join(chartDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, content, 0644))
	}

	valuesEnvPath := filepath.Join(tempDir, "values-env.yaml")

	// 1. Test deploy.yaml rendering
	cmd := exec.Command("helm", "template", "test-sidecar-vols", chartDir, "-f", valuesEnvPath, "-s", "templates/deploy.yaml")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template deploy.yaml failed: %s", string(out))

	renderedDeploy := string(out)
	// setup init container mounts
	assert.Contains(t, renderedDeploy, "- name: main-vol\n              mountPath: /shared/data")
	assert.Contains(t, renderedDeploy, "- name: init-setup-init-scratch\n              mountPath: /scratch")

	// sidecar extraContainer mounts
	assert.Contains(t, renderedDeploy, "- name: main-vol\n              mountPath: /sidecar/shared\n              readOnly: true")
	assert.Contains(t, renderedDeploy, "- name: sidecar-sidecar-sidecar-pvc\n              mountPath: /sidecar/storage")

	// bare containers should not have volumeMounts
	assert.NotContains(t, renderedDeploy, "- name: bare-init\n          volumeMounts:")
	assert.NotContains(t, renderedDeploy, "- name: bare-sidecar\n          volumeMounts:")

	// Pod-level volumes
	assert.Contains(t, renderedDeploy, "- name: init-setup-init-scratch\n          emptyDir:\n            {}")
	assert.Contains(t, renderedDeploy, "- name: sidecar-sidecar-sidecar-pvc\n          persistentVolumeClaim:\n            claimName: test-sidecar-vols-core-sidecar-sidecar-pvc")
	assert.Contains(t, renderedDeploy, "- name: main-vol\n          persistentVolumeClaim:\n            claimName: test-sidecar-vols-core-main-vol")

	// 2. Test pvc.yaml rendering
	cmdPVC := exec.Command("helm", "template", "test-sidecar-vols", chartDir, "-f", valuesEnvPath, "-s", "templates/pvc.yaml")
	outPVC, errPVC := cmdPVC.CombinedOutput()
	assert.NoError(t, errPVC, "helm template pvc.yaml failed: %s", string(outPVC))

	renderedPVC := string(outPVC)
	assert.Contains(t, renderedPVC, "name: test-sidecar-vols-core-main-vol")
	assert.Contains(t, renderedPVC, "storage: 5Gi")
	assert.Contains(t, renderedPVC, "name: test-sidecar-vols-core-sidecar-sidecar-pvc")
	assert.Contains(t, renderedPVC, "storage: 2Gi")
	// ephemeral init-scratch must not generate a PVC
	assert.NotContains(t, renderedPVC, "init-scratch")
}

func TestHandleGenerate_ReleaseNamePrefixStripping(t *testing.T) {
	manifests := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: release-name-smd-roles
  labels:
    app.kubernetes.io/name: smd
    app.kubernetes.io/instance: release-name
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: smd
  template:
    metadata:
      labels:
        app.kubernetes.io/name: smd
    spec:
      containers:
      - name: roles
        image: quay.io/seplan/smd:roles-1.0
        ports:
        - containerPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: release-name-smd-web
  labels:
    app.kubernetes.io/name: smd
    app.kubernetes.io/instance: release-name
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: smd
  template:
    metadata:
      labels:
        app.kubernetes.io/name: smd
    spec:
      containers:
      - name: web
        image: quay.io/seplan/smd:web-1.0
        ports:
        - containerPort: 80
`

	req := httptest.NewRequest(http.MethodPost, "/v1/generate", bytes.NewBufferString(manifests))
	req.Header.Set("X-Chart-Name", "smd")
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
	// Component names should be stripped cleanly to 'roles' and 'web', NOT 'release-name-smd-roles'
	assert.Contains(t, valuesEnv, "  roles:\n")
	assert.Contains(t, valuesEnv, "  web:\n")
	assert.NotContains(t, valuesEnv, "release-name-smd")

	chartValues := files["chart/values.yaml"]
	assert.Contains(t, chartValues, "    roles:\n")
	assert.Contains(t, chartValues, "    web:\n")
	assert.NotContains(t, chartValues, "release-name-smd")
}

func TestServiceEnabled_WorkerWithoutService(t *testing.T) {
	disabled := false
	enabled := true
	wizardParams := helm.WizardParams{
		ChartName: "test-svc-enabled",
		Deployments: map[string]helm.DeploymentParams{
			"web": {
				Image: helm.ImageParams{Repository: "test/web", Tag: "1.0"},
				Service: helm.ServiceParams{
					Enabled: &enabled,
					Ports: map[string]struct {
						Port          int    `json:"port" yaml:"port"`
						ContainerPort int    `json:"containerPort,omitempty" yaml:"containerPort,omitempty"`
						Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
					}{
						"http": {Port: 8080},
					},
				},
				Routes: map[string]helm.SubRouteParams{
					"default": {Enabled: true, Host: "web.example.com"},
				},
			},
			"worker": {
				// Pure background worker with service.enabled: false and no ports
				Image: helm.ImageParams{Repository: "test/worker", Tag: "1.0"},
				Service: helm.ServiceParams{
					Enabled: &disabled,
				},
				Routes: map[string]helm.SubRouteParams{
					"default": {Enabled: true, Host: "worker.example.com"},
				},
			},
		},
	}

	files, err := helm.GenerateWizardChart(wizardParams)
	assert.NoError(t, err)

	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	for path, content := range files {
		if path == "values-env.yaml" {
			assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "values-env.yaml"), content, 0644))
			continue
		}
		targetPath := filepath.Join(chartDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, content, 0644))
	}

	valuesEnvPath := filepath.Join(tempDir, "values-env.yaml")

	// 1. Verify svc.yaml: 'web' must have a Service; 'worker' must NOT have a Service
	cmdSVC := exec.Command("helm", "template", "test-svc-enabled", chartDir, "-f", valuesEnvPath, "-s", "templates/svc.yaml")
	outSVC, errSVC := cmdSVC.CombinedOutput()
	assert.NoError(t, errSVC, "helm template svc.yaml failed: %s", string(outSVC))

	renderedSVC := string(outSVC)
	assert.Contains(t, renderedSVC, "test-svc-enabled-web")
	assert.NotContains(t, renderedSVC, "test-svc-enabled-worker")

	// 2. Verify deploy.yaml: 'web' must have ports; 'worker' must NOT have ports
	cmdDeploy := exec.Command("helm", "template", "test-svc-enabled", chartDir, "-f", valuesEnvPath, "-s", "templates/deploy.yaml")
	outDeploy, errDeploy := cmdDeploy.CombinedOutput()
	assert.NoError(t, errDeploy, "helm template deploy.yaml failed: %s", string(outDeploy))

	renderedDeploy := string(outDeploy)
	assert.Contains(t, renderedDeploy, "containerPort: 8080")
	// worker deployment should not render container ports
	docs := strings.Split(renderedDeploy, "---")
	var workerDeploy string
	for _, doc := range docs {
		if strings.Contains(doc, "name: test-svc-enabled-worker") {
			workerDeploy = doc
			break
		}
	}
	assert.NotEmpty(t, workerDeploy, "rendered deploy:\n%s", renderedDeploy)
	assert.NotContains(t, workerDeploy, "ports:")

	// 3. Verify route.yaml: 'web' must have a Route; 'worker' must NOT have a Route
	cmdRoute := exec.Command("helm", "template", "test-svc-enabled", chartDir, "-f", valuesEnvPath, "-s", "templates/route.yaml")
	outRoute, errRoute := cmdRoute.CombinedOutput()
	assert.NoError(t, errRoute, "helm template route.yaml failed: %s", string(outRoute))

	renderedRoute := string(outRoute)
	assert.Contains(t, renderedRoute, "host: \"web.example.com\"")
	assert.NotContains(t, renderedRoute, "worker.example.com")
}

func TestSingleDeployment_NoDuplicateChartSuffix(t *testing.T) {
	manifests := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: smd
  labels:
    app: smd
spec:
  replicas: 1
  selector:
    matchLabels:
      app: smd
  template:
    metadata:
      labels:
        app: smd
    spec:
      containers:
      - name: smd
        image: quay.io/tjpa/smd:1.0.0
        ports:
        - containerPort: 8080
`
	req := httptest.NewRequest(http.MethodPost, "/v1/generate", bytes.NewBufferString(manifests))
	req.Header.Set("X-Chart-Name", "smd")
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
	// Verify VSO role and path are NOT duplicated as smd-smd-role or kv/data/smd/smd
	assert.Contains(t, valuesEnv, `role: "smd-role"`)
	assert.Contains(t, valuesEnv, `path: "kv/data/smd"`)
	assert.NotContains(t, valuesEnv, "smd-smd-role")
	assert.NotContains(t, valuesEnv, "kv/data/smd/smd")

	// Verify podAntiAffinity does not duplicate name
	assert.NotContains(t, valuesEnv, "- smd-smd")
	t.Logf("chart/values.yaml:\n%s", files["chart/values.yaml"])

	// Verify that the chart renders cleanly with ONLY default values.yaml (without values-env.yaml)
	tempDir := t.TempDir()
	for path, content := range files {
		targetPath := filepath.Join(tempDir, path)
		if strings.HasSuffix(path, "/") {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}
		if len(content) == 0 && (path == "chart" || path == "chart/templates" || path == "chart/files") {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, []byte(content), 0644))
	}
	cmd := exec.Command("helm", "template", "smd", filepath.Join(tempDir, "chart"))
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template with default values failed: %s", string(out))
}

func TestGenerate_IntegracaoPjeTpu_AllTemplates_DefaultValues(t *testing.T) {
	manifests := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: integracao-pje-tpu
spec:
  replicas: 1
  selector:
    matchLabels:
      app: integracao-pje-tpu
  template:
    metadata:
      labels:
        app: integracao-pje-tpu
    spec:
      containers:
        - name: integracao-pje-tpu
          image: tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/ca/sistemas-judiciais/satelites/integracao-pje-tpu:hml-34111dbd
          ports:
            - name: http
              containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: integracao-pje-tpu
spec:
  ports:
    - name: http
      port: 8080
      targetPort: 8080
  selector:
    app: integracao-pje-tpu
`

	req := httptest.NewRequest(http.MethodPost, "/v1/generate", strings.NewReader(manifests))
	req.Header.Set("Content-Type", "text/yaml")
	req.Header.Set("X-Chart-Name", "integracao-pje-tpu")
	req.Header.Set("X-Generate-All-Templates", "true")
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

	// Verify chart/values.yaml has the global baseline
	valuesYaml := files["chart/values.yaml"]
	assert.Contains(t, valuesYaml, "global:")
	assert.Contains(t, valuesYaml, "config:")
	assert.Contains(t, valuesYaml, "secrets:")

	// Verify that the chart renders cleanly with ONLY default values.yaml (no values-env.yaml)
	tempDir := t.TempDir()
	for path, content := range files {
		targetPath := filepath.Join(tempDir, path)
		if strings.HasSuffix(path, "/") {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}
		if len(content) == 0 && (path == "chart" || path == "chart/templates" || path == "chart/files") {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, []byte(content), 0644))
	}

	cmd := exec.Command("helm", "template", "integracao-pje-tpu", filepath.Join(tempDir, "chart"))
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template with default values failed: %s", string(out))
}

func TestSubcomponents_ParityAndDefaultValues(t *testing.T) {
	wizardParams := helm.WizardParams{
		ChartName:  "myapp",
		DevRepoURL: "https://gitlab.example.com/myapp.git",
		Deployments: map[string]helm.DeploymentParams{
			"myapp": {
				Image: helm.ImageParams{
					Repository: "quay.io/tjpa/myapp",
					Tag:        "latest",
				},
				Service: helm.ServiceParams{
					Type: "ClusterIP",
					Port: func() *int { p := 8080; return &p }(),
				},
			},
		},
		Subcomponents: []string{"postgres", "redis"},
	}

	files, err := helm.GenerateWizardChart(wizardParams)
	assert.NoError(t, err)

	// Verify values.yaml has subcomponents enabled
	valuesYaml := string(files["values.yaml"])
	assert.Contains(t, valuesYaml, "postgres:")
	assert.Contains(t, valuesYaml, "redis:")
	assert.Contains(t, valuesYaml, "global:")

	// Verify values-env.yaml has modern volumes structure and no legacy persistence in subcomponents
	valuesEnv := string(files["values-env.yaml"])
	assert.NotContains(t, valuesEnv, "postgres:\n  persistence:")
	assert.NotContains(t, valuesEnv, "redis:\n  persistence:")
	assert.Contains(t, valuesEnv, "volumes:\n    data:\n      enabled: true\n      mountPath: \"/var/lib/postgresql/data\"")
	assert.Contains(t, valuesEnv, "volumes:\n    data:\n      enabled: true\n      mountPath: \"/data\"")

	// Verify templates exist
	postgresTmpl, hasPg := files["templates/postgres.yaml"]
	assert.True(t, hasPg)
	assert.Contains(t, string(postgresTmpl), "(ne .enabled false)")
	assert.Contains(t, string(postgresTmpl), "-postgres-sa")
	assert.Contains(t, string(postgresTmpl), "-postgres-vault-auth")

	redisTmpl, hasRedis := files["templates/redis.yaml"]
	assert.True(t, hasRedis)
	assert.Contains(t, string(redisTmpl), "(ne .enabled false)")
	assert.Contains(t, string(redisTmpl), "-redis-sa")
	assert.Contains(t, string(redisTmpl), "-redis-vault-auth")

	// Render with helm template using default values.yaml
	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	assert.NoError(t, os.MkdirAll(chartDir, 0755))
	for path, content := range files {
		targetPath := filepath.Join(chartDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, content, 0644))
	}

	cmd := exec.Command("helm", "template", "myapp", chartDir)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template on chart with default values failed: %s", string(out))

	// Render with helm template using values-env.yaml (which includes subcomponents values-snippets)
	cmdEnv := exec.Command("helm", "template", "myapp", chartDir, "-f", filepath.Join(chartDir, "values-env.yaml"))
	outEnv, err := cmdEnv.CombinedOutput()
	assert.NoError(t, err, "helm template with values-env.yaml failed: %s", string(outEnv))

	renderedEnv := string(outEnv)
	assert.Contains(t, renderedEnv, "name: myapp-postgres")
	assert.Contains(t, renderedEnv, "mountPath: /var/lib/postgresql/data")
	assert.Contains(t, renderedEnv, "mountPath: /data")

	// Verify that subcomponent templates render safely when disabled or missing from values
	cmdDisabled := exec.Command("helm", "template", "myapp", chartDir, "--set", "postgres.enabled=false", "--set", "redis.enabled=false")
	outDisabled, errDisabled := cmdDisabled.CombinedOutput()
	assert.NoError(t, errDisabled, "helm template with subcomponents disabled failed: %s", string(outDisabled))

	// Verify that subcomponents consume shared resources properly
	cmdShared := exec.Command("helm", "template", "myapp", chartDir,
		"--set", "shared.config.env.common-tuning.MAX_CONN=200",
		"--set", "shared.secrets.files.shared-ca.content=rootCA",
		"--set", "shared.secrets.files.shared-ca.mountPath=/etc/ssl/certs/ca-bundle.crt",
		"--set", "postgres.shared.config.env[0]=common-tuning",
		"--set", "postgres.shared.secrets.files[0]=shared-ca",
		"--set", "redis.shared.config.env[0]=common-tuning",
		"--set", "redis.shared.secrets.files[0]=shared-ca",
	)
	outShared, errShared := cmdShared.CombinedOutput()
	assert.NoError(t, errShared, "helm template with subcomponents shared resources failed: %s", string(outShared))
	renderedShared := string(outShared)
	assert.Contains(t, renderedShared, "name: myapp-shared-common-tuning")
	assert.Contains(t, renderedShared, "name: shared-sec-shared-ca")
	assert.Contains(t, renderedShared, "mountPath: /etc/ssl/certs/ca-bundle.crt")
}

func TestWorkloadGovernance_SecurityViolationGuards(t *testing.T) {
	wizardParams := helm.WizardParams{
		ChartName:  "secureapp",
		DevRepoURL: "https://gitlab.example.com/secureapp.git",
		Deployments: map[string]helm.DeploymentParams{
			"secureapp": {
				Image: helm.ImageParams{Repository: "quay.io/tjpa/secureapp", Tag: "1.0.0"},
				ExtraContainers: map[string]*helm.SidecarParams{
					"authorized-sidecar": {
						Image: helm.ImageParams{Repository: "quay.io/tjpa/sidecar", Tag: "latest"},
					},
				},
				InitContainers: map[string]*helm.SidecarParams{
					"authorized-init": {
						Image: helm.ImageParams{Repository: "quay.io/tjpa/init", Tag: "latest"},
					},
				},
			},
		},
		CronJobs: map[string]helm.DeploymentParams{
			"authorized-cron": {
				Schedule: "0 0 * * *",
				Image:    helm.ImageParams{Repository: "quay.io/tjpa/cron", Tag: "latest"},
			},
		},
	}

	files, err := helm.GenerateWizardChart(wizardParams)
	assert.NoError(t, err)

	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	assert.NoError(t, os.MkdirAll(chartDir, 0755))
	for path, content := range files {
		targetPath := filepath.Join(chartDir, path)
		assert.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
		assert.NoError(t, os.WriteFile(targetPath, content, 0644))
	}

	// Verify generated values contain allowedWorkloads
	assert.Contains(t, string(files["values.yaml"]), "allowedWorkloads:")
	assert.Contains(t, string(files["values-env.yaml"]), "allowedWorkloads:")

	// 1. Authorized configuration should pass
	cmdPass := exec.Command("helm", "template", "secureapp", chartDir)
	outPass, errPass := cmdPass.CombinedOutput()
	assert.NoError(t, errPass, "helm template should succeed with authorized components: %s", string(outPass))

	// 2. Unauthorized deployment should fail with SECURITY VIOLATION
	cmdRogueDeploy := exec.Command("helm", "template", "secureapp", chartDir, "--set", "deploys.rogue-deploy.image.repository=evil/miner")
	outRogueDeploy, errRogueDeploy := cmdRogueDeploy.CombinedOutput()
	assert.Error(t, errRogueDeploy)
	assert.Contains(t, string(outRogueDeploy), "SECURITY VIOLATION")
	assert.Contains(t, string(outRogueDeploy), "Deployment 'rogue-deploy' is not authorized in chart 'secureapp'")

	// 3. Unauthorized cronjob should fail with SECURITY VIOLATION
	cmdRogueCron := exec.Command("helm", "template", "secureapp", chartDir, "--set", "cronjobs.rogue-cron.schedule=* * * * *")
	outRogueCron, errRogueCron := cmdRogueCron.CombinedOutput()
	assert.Error(t, errRogueCron)
	assert.Contains(t, string(outRogueCron), "SECURITY VIOLATION")
	assert.Contains(t, string(outRogueCron), "CronJob 'rogue-cron' is not authorized in chart 'secureapp'")

	// 4. Granular container governance (map format in allowedWorkloads)
	containerGovValues := `
allowedWorkloads:
  deploys:
    secureapp:
      extraContainers:
        - authorized-sidecar
      initContainers:
        - authorized-init
  cronjobs:
    - authorized-cron
`
	valuesFile := filepath.Join(tempDir, "values-container-gov.yaml")
	assert.NoError(t, os.WriteFile(valuesFile, []byte(containerGovValues), 0644))

	// Unauthorized extraContainer should fail with SECURITY VIOLATION
	cmdRogueSidecar := exec.Command("helm", "template", "secureapp", chartDir, "-f", valuesFile, "--set", "deploys.secureapp.extraContainers.rogue-sidecar.image.repository=evil/miner")
	outRogueSidecar, errRogueSidecar := cmdRogueSidecar.CombinedOutput()
	assert.Error(t, errRogueSidecar)
	assert.Contains(t, string(outRogueSidecar), "SECURITY VIOLATION")
	assert.Contains(t, string(outRogueSidecar), "ExtraContainer (sidecar) 'rogue-sidecar' is not authorized in deployment 'secureapp'")

	// Unauthorized initContainer should fail with SECURITY VIOLATION
	cmdRogueInit := exec.Command("helm", "template", "secureapp", chartDir, "-f", valuesFile, "--set", "deploys.secureapp.initContainers.rogue-init.image.repository=evil/miner")
	outRogueInit, errRogueInit := cmdRogueInit.CombinedOutput()
	assert.Error(t, errRogueInit)
	assert.Contains(t, string(outRogueInit), "SECURITY VIOLATION")
	assert.Contains(t, string(outRogueInit), "InitContainer 'rogue-init' is not authorized in deployment 'secureapp'")
}

func TestUniversalModelSharedResources(t *testing.T) {
	chartDir := "../../models/universal"

	testValues := `
allowedWorkloads:
  deploys:
    - api
    - app
  cronjobs: []

shared:
  config:
    env:
      timezone-cm:
        env:
          TZ: "America/Belem"
    files:
      app-props:
        mountPath: /app/config/application.properties
        content: "timeout=5000"
  secrets:
    env:
      pje-db:
        env:
          DB_USER: "smd_app"
          DB_PASSWORD: "secretpassword"
    files:
      pje-cert:
        mountPath: /app/certs/cert.p12
        content: "Y2VydGlmaWNhdGUtZGF0YQ=="
        b64enc: true

deploys:
  api:
    shared:
      config:
        env:
          - name: timezone-cm
        files:
          - name: app-props
      secrets:
        env:
          - name: pje-db
        files:
          - name: pje-cert

  app:
    shared:
      secrets:
        files:
          - name: pje-cert
            mountPath: /opt/custom/cert.p12
`

	valuesFile := filepath.Join(t.TempDir(), "values-test.yaml")
	assert.NoError(t, os.WriteFile(valuesFile, []byte(testValues), 0644))

	cmd := exec.Command("helm", "template", "myrelease", chartDir, "-f", valuesFile)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "helm template should succeed: %s", string(out))

	outStr := string(out)

	// Verify shared Secret & ConfigMap manifests are rendered
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-pje-db")
	assert.Contains(t, outStr, "DB_USER: \"smd_app\"")
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-pje-cert")
	assert.Contains(t, outStr, "pje-cert: \"Y2VydGlmaWNhdGUtZGF0YQ==\"")
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-timezone-cm")
	assert.Contains(t, outStr, "TZ: \"America/Belem\"")
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-app-props")
	assert.Contains(t, outStr, "timeout=5000")

	// Verify API deployment envFrom & mounts
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-timezone-cm")
	assert.Contains(t, outStr, "name: myrelease-chart-model-multi-shared-pje-db")
	assert.Contains(t, outStr, "mountPath: /app/certs/cert.p12")
	assert.Contains(t, outStr, "mountPath: /app/config/application.properties")

	// Verify APP deployment mounts the same shared secret at the overridden path
	assert.Contains(t, outStr, "mountPath: /opt/custom/cert.p12")
	assert.Contains(t, outStr, "secretName: myrelease-chart-model-multi-shared-pje-cert")
}



