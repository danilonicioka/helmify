package app

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var portNameRegex = regexp.MustCompile(`^(\d+)-(tcp|udp)$`)

// sanitizeObject pre-processes and cleans K8s manifests prior to Helmify translation.
// Returns false if the object should be discarded entirely.
func sanitizeObject(obj *unstructured.Unstructured) bool {
	if strings.ToLower(obj.GetKind()) == "namespace" {
		return false
	}

	// Remove system/status fields that shouldn't be in a Helm chart
	delete(obj.Object, "status")
	if metadata, ok := obj.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "creationTimestamp")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "generation")
		delete(metadata, "selfLink")
		delete(metadata, "managedFields")
	}

	// Normalize port names throughout the object
	normalizePorts(obj.Object)

	return true
}

func normalizePortName(name string) string {
	matches := portNameRegex.FindStringSubmatch(strings.ToLower(name))
	if len(matches) == 3 {
		return matches[2] + "-" + matches[1]
	}
	return name
}

func normalizePorts(obj interface{}) {
	switch val := obj.(type) {
	case map[string]interface{}:
		if portNameVal, exists := val["name"]; exists {
			if name, ok := portNameVal.(string); ok {
				val["name"] = normalizePortName(name)
			}
		}
		if targetPortVal, exists := val["targetPort"]; exists {
			if targetPortStr, ok := targetPortVal.(string); ok {
				val["targetPort"] = normalizePortName(targetPortStr)
			}
		}
		for _, v := range val {
			normalizePorts(v)
		}
	case []interface{}:
		for _, v := range val {
			normalizePorts(v)
		}
	}
}

// cleanKomposeMetadata deeply iterates through a K8s object and removes any
// label, annotation, or selector key starting with "io.kompose." or "kompose."
func cleanKomposeMetadata(obj *unstructured.Unstructured) {
	cleanMap(obj.Object)
}

func cleanMap(obj interface{}) {
	switch m := obj.(type) {
	case map[string]interface{}:
		// Clean the common metadata/selector containers if they exist at this level
		cleanKeys(m, "labels")
		cleanKeys(m, "annotations")
		cleanKeys(m, "selector")
		cleanKeys(m, "matchLabels")

		// Recurse into all maps
		for _, v := range m {
			cleanMap(v)
		}
	case []interface{}:
		for _, v := range m {
			cleanMap(v)
		}
	}
}

func cleanKeys(m map[string]interface{}, containerKey string) {
	if container, ok := m[containerKey].(map[string]interface{}); ok {
		for k := range container {
			if strings.HasPrefix(k, "io.kompose.") || strings.HasPrefix(k, "kompose.") {
				delete(container, k)
			}
		}
		// If the container is now empty, we delete the container entirely
		// to keep the yaml clean, unless it's an empty selector which some apps might require?
		// Typically an empty label map is fine to be deleted so it doesn't render "labels: {}"
		if len(container) == 0 {
			delete(m, containerKey)
		}
	}
}

