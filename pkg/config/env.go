package config

import (
	"os"
	"strings"

	roothelmify "github.com/danilonicioka/helmify"
)

type EnvConfig struct {
	OrgName           string `json:"orgName"`
	DefaultTimezone   string `json:"defaultTimezone"`
	Registry          string `json:"registry"`
	DefaultDomain     string `json:"defaultDomain"`
	InternalDomain    string `json:"internalDomain"`
	ExternalDomain    string `json:"externalDomain"`
	ChartVersion      string `json:"chartVersion"`
	GitlabCIPath      string `json:"gitlabCIPath"`
}

var GlobalEnvConfig EnvConfig

func LoadEnvConfig() {
	GlobalEnvConfig = EnvConfig{
		OrgName:           getEnvOrDefault("HELMIFY_ORG_NAME", "Standardized Organization"),
		DefaultTimezone:   getEnvOrDefault("HELMIFY_DEFAULT_TIMEZONE", "UTC"),
		Registry:          getEnvOrDefault("HELMIFY_REGISTRY", "registry.example.com/devops"),
		DefaultDomain:     getEnvOrDefault("HELMIFY_DEFAULT_DOMAIN", "apps.example.com"),
		InternalDomain:    getEnvOrDefault("HELMIFY_INTERNAL_DOMAIN", "internal.example.com"),
		ExternalDomain:    getEnvOrDefault("HELMIFY_EXTERNAL_DOMAIN", "example.com"),
		ChartVersion:      getEnvOrDefault("HELMIFY_CHART_VERSION", strings.TrimSpace(roothelmify.ChartVersion)),
		GitlabCIPath:      getEnvOrDefault("HELMIFY_GITLAB_CI_PATH", "/app/files/.gitlab-ci.yml"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
