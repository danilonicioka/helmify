package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateWizardChart_Single(t *testing.T) {
	params := WizardParams{
		ChartName: "test-single-app",
		Type:      "single",
		GlobalConfig: map[string]string{
			"TZ": "America/Sao_Paulo",
		},
		Deployments: map[string]DeploymentParams{
			"test-single-app": {
				Replicas: intPtr(3),
				Image: ImageParams{
					Repository: "quay.io/tjpa/my-app",
					Tag:        "v2.1.0",
				},
				Service: ServiceParams{
					Port: 9090,
				},
				Cm: map[string]string{
					"VAR_A": "VAL_A",
				},
				Secret: map[string]string{
					"SECRET_B": "VAL_B",
				},
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
	assert.Contains(t, valuesStr, "repository: quay.io/tjpa/my-app")
	assert.Contains(t, valuesStr, "tag: v2.1.0")
	assert.Contains(t, valuesStr, "port: 9090")
	assert.Contains(t, valuesStr, "VAR_A: VAL_A")
	assert.Contains(t, valuesStr, "SECRET_B: VAL_B")
	assert.Contains(t, valuesStr, "path: /prefix")
	assert.Contains(t, valuesStr, "default.host.com")
	assert.Contains(t, valuesStr, "internal.host.com")
	assert.Contains(t, valuesStr, "TZ: America/Sao_Paulo")

	// Check deployment.yaml has renamed references
	deployBytes, ok := files["templates/deployment.yaml"]
	assert.True(t, ok)
	assert.Contains(t, string(deployBytes), "test-single-app.fullname")
}

func TestGenerateWizardChart_Multi(t *testing.T) {
	params := WizardParams{
		ChartName: "test-multi-app",
		Type:      "multi",
		Deployments: map[string]DeploymentParams{
			"api": {
				Replicas: intPtr(2),
				Image: ImageParams{
					Repository: "quay.io/tjpa/api-app",
					Tag:        "v1.0.0",
				},
				Service: ServiceParams{
					Port: 8080,
				},
				Route: RouteParams{
					Default: SubRouteParams{
						Enabled: true,
						Host:    "api.default.host.com",
					},
				},
			},
			"bff": {
				Replicas: intPtr(1),
				Image: ImageParams{
					Repository: "quay.io/tjpa/bff-app",
					Tag:        "v1.1.0",
				},
				Service: ServiceParams{
					Port: 5000,
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
	assert.Contains(t, valuesStr, "api:")
	assert.Contains(t, valuesStr, "bff:")
	assert.NotContains(t, valuesStr, "web:") // web should be deleted because it wasn't requested

	// Check if bff templates are created
	_, ok = files["templates/deploy-api.yaml"]
	assert.True(t, ok)
	_, ok = files["templates/deploy-bff.yaml"]
	assert.True(t, ok)
	_, ok = files["templates/deploy-web.yaml"]
	assert.False(t, ok) // web templates should be deleted
}

func TestGetModelDefaults(t *testing.T) {
	t.Run("single defaults", func(t *testing.T) {
		defaults, err := GetModelDefaults("single")
		assert.NoError(t, err)
		assert.NotNil(t, defaults)
		assert.Contains(t, defaults, "global")
		assert.Contains(t, defaults, "chart-model-single")
	})

	t.Run("multi defaults", func(t *testing.T) {
		defaults, err := GetModelDefaults("multi")
		assert.NoError(t, err)
		assert.NotNil(t, defaults)
		assert.Contains(t, defaults, "global")
		assert.Contains(t, defaults, "api")
		assert.Contains(t, defaults, "web")
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := GetModelDefaults("invalid")
		assert.Error(t, err)
	})
}

func intPtr(val int) *int {
	return &val
}

func TestGenerateWizardChart_ZeroReplicas(t *testing.T) {
	params := WizardParams{
		ChartName: "test-zero-replicas",
		Type:      "single",
		Deployments: map[string]DeploymentParams{
			"test-zero-replicas": {
				Replicas: intPtr(0),
				Image: ImageParams{
					Repository: "quay.io/tjpa/my-app",
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

