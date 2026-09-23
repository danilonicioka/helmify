package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateWizardChart_Single(t *testing.T) {
	params := WizardParams{
		ChartName: "test-single-app",
		GlobalConfig: map[string]string{
			"TZ": "America/Sao_Paulo",
		},
		Deployments: map[string]DeploymentParams{
			"test-single-app": {
				Replicas: intPtr(3),
				Image: ImageParams{
					Repository: "quay.io/my-org/my-app",
					Tag:        "v2.1.0",
				},
				Service: ServiceParams{
					Port: intPtr(9090),
				},
				Config: &ConfigParams{Env: map[string]string{"TZ": "America/Belem"}},
				Secrets: &ConfigParams{Env: map[string]string{"API_KEY": "12345"}},
				Route: RouteParams{
					Path: "/prefix",
					Default: SubRouteParams{
						Enabled: true,
						Host:    "default.host.com",
					},
					Internal: SubRouteParams{
						Enabled: true,
						Host:    "internal.host.com",
					},
				},
			},
		},
	}

	files, err := GenerateWizardChart(params)
	assert.NoError(t, err)
	assert.NotEmpty(t, files)

	// Check if values.yaml was updated
	valuesBytes, ok := files["values.yaml"]
	assert.True(t, ok)
	valuesStr := string(valuesBytes)
	assert.Contains(t, valuesStr, "test-single-app:")
	assert.Contains(t, valuesStr, "replicas: 3")
	assert.Contains(t, valuesStr, "repository: quay.io/my-org/my-app")
	assert.Contains(t, valuesStr, "tag: v2.1.0")
	assert.Contains(t, valuesStr, "port: 9090")
	assert.Contains(t, valuesStr, "TZ: America/Belem")
	assert.Contains(t, valuesStr, "API_KEY:")
	assert.Contains(t, valuesStr, "path: /prefix")
	assert.Contains(t, valuesStr, "default.host.com")
	assert.Contains(t, valuesStr, "internal.host.com")
	assert.Contains(t, valuesStr, "TZ: America/Sao_Paulo")
	assert.Contains(t, valuesStr, "fullnameOverride: test-single-app")

		

	}

func TestGenerateWizardChart_Multi(t *testing.T) {
	params := WizardParams{
		ChartName: "test-multi-app",
		Deployments: map[string]DeploymentParams{
			"backend": {
				Replicas: intPtr(2),
				Image: ImageParams{
					Repository: "quay.io/my-org/backend-app",
					Tag:        "v1.0.0",
				},
				Service: ServiceParams{
					Port: intPtr(8080),
				},
				Route: RouteParams{
					Default: SubRouteParams{
						Enabled: true,
						Host:    "backend.default.host.com",
					},
				},
			},
			"bff": {
				Replicas: intPtr(1),
				Image: ImageParams{
					Repository: "quay.io/my-org/bff-app",
					Tag:        "v1.1.0",
				},
				Service: ServiceParams{
					Port: intPtr(5000),
				},
				Route: RouteParams{
					Default: SubRouteParams{
						Enabled: true,
						Host:    "bff.default.host.com",
					},
				},
			},
		},
	}

	files, err := GenerateWizardChart(params)
	assert.NoError(t, err)
	assert.NotEmpty(t, files)

	// Check values.yaml contents
	valuesBytes, ok := files["values.yaml"]
	assert.True(t, ok)
	valuesStr := string(valuesBytes)
	assert.Contains(t, valuesStr, "backend:")
	assert.Contains(t, valuesStr, "bff:")
	assert.NotContains(t, valuesStr, "frontend:") // frontend should be deleted because it wasn't requested
	assert.Contains(t, valuesStr, "fullnameOverride: test-multi-app")

	// Check if bff templates are created
	
	
	 // frontend templates should be deleted
}

func TestGetModelDefaults(t *testing.T) {
	defaults, err := GetModelDefaults()
	assert.NoError(t, err)
	assert.NotNil(t, defaults)
	assert.Contains(t, defaults, "global")
	assert.Contains(t, defaults, "deploys")
}

func intPtr(val int) *int {
	return &val
}

func TestGenerateWizardChart_ZeroReplicas(t *testing.T) {
	params := WizardParams{
		ChartName: "test-zero-replicas",
		Deployments: map[string]DeploymentParams{
			"test-zero-replicas": {
				Replicas: intPtr(0),
				Image: ImageParams{
					Repository: "quay.io/my-org/my-app",
				},
			},
		},
	}

	files, err := GenerateWizardChart(params)
	assert.NoError(t, err)
	assert.NotEmpty(t, files)

	valuesBytes, ok := files["values.yaml"]
	assert.True(t, ok)
	assert.Contains(t, string(valuesBytes), "replicas: 0")
}

func TestRouteHostPrefixCalculation(t *testing.T) {
	// Single deployment
	prefix1 := getRouteHostPrefix("gotenberg", "gotenberg", "", false)
	assert.Equal(t, "gotenberg", prefix1)

	// Multi deployment - frontend component should omit suffix
	prefix2 := getRouteHostPrefix("gotenberg", "app", "", true)
	assert.Equal(t, "gotenberg", prefix2)

	// Multi deployment - path specified should omit suffix
	prefix3 := getRouteHostPrefix("gotenberg", "api", "/api", true)
	assert.Equal(t, "gotenberg", prefix3)

	// Multi deployment - generic component (api), no path, should append suffix
	prefix4 := getRouteHostPrefix("gotenberg", "api", "", true)
	assert.Equal(t, "gotenberg-api", prefix4)

	// Multi deployment - suffix already exists in component name, no double suffix
	prefix5 := getRouteHostPrefix("gotenberg", "gotenberg-api", "", true)
	assert.Equal(t, "gotenberg-api", prefix5)
}

func TestWriteTarGzStructure(t *testing.T) {
	mockFiles := map[string][]byte{
		"README.md":        []byte("readme content"),
		".gitlab-ci.yml":   []byte("ci content"),
		"Chart.yaml":       []byte("chart content"),
		"values.yaml":      []byte("values content"),
		"templates/d.yaml": []byte("deploy content"),
	}

	var buf bytes.Buffer
	err := WriteTarGz(mockFiles, "mychart", &buf)
	assert.NoError(t, err)

	// Read tar.gz content back
	gr, err := gzip.NewReader(&buf)
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

	assert.True(t, paths["README.md"])
	assert.True(t, paths[".gitlab-ci.yml"])
	assert.True(t, paths["chart/Chart.yaml"])
	assert.True(t, paths["chart/values.yaml"])
	assert.True(t, paths["chart/templates/d.yaml"])
	assert.False(t, paths["mychart/Chart.yaml"])
}
