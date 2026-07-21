package route

import (
	"bytes"
	"testing"

	"github.com/arttor/helmify/internal"
	"github.com/arttor/helmify/pkg/config"
	"github.com/arttor/helmify/pkg/metadata"
	"github.com/stretchr/testify/assert"
)

const routeYaml = `apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: my-app-api
  namespace: my-namespace
  annotations:
    my-annotation: annotation-value
spec:
  host: my-app-api.apps.ocp-hub.i.tj.pa.gov.br
  path: /api
  to:
    kind: Service
    name: my-app-api-service
  port:
    targetPort: 8080
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect`

func Test_route_Process(t *testing.T) {
	var testInstance route

	t.Run("processed", func(t *testing.T) {
		obj := internal.GenerateObj(routeYaml)
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, true, processed)
		assert.NotNil(t, template)

		// Check values
		values := template.Values()
		assert.NotEmpty(t, values)

		var data string
		if rr, ok := template.(interface{ Data() string }); ok {
			data = rr.Data()
		} else {
			var buf bytes.Buffer
			err = template.Write(&buf)
			assert.NoError(t, err)
			data = buf.String()
		}

		assert.Contains(t, data, "apiVersion: route.openshift.io/v1")
		assert.Contains(t, data, "kind: Route")
		assert.Contains(t, data, "default.host")
		assert.Contains(t, data, "internal.host")
		assert.Contains(t, data, "external.host")
	})

	t.Run("skipped-ext", func(t *testing.T) {
		obj := internal.GenerateObj(`apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: my-app-api-ext`)
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, true, processed)
		assert.Nil(t, template)
	})

	t.Run("skipped-non-route", func(t *testing.T) {
		obj := internal.TestNs
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, false, processed)
		assert.Nil(t, template)
	})

	t.Run("templated target service name matches service suffix", func(t *testing.T) {
		appMeta := metadata.New(config.Config{ChartName: "my-app"})

		// Load a Service resource matching the target service name
		serviceYaml := `apiVersion: v1
kind: Service
metadata:
  name: my-app-api-service
spec:
  ports:
  - port: 8080`
		svcObj := internal.GenerateObj(serviceYaml)
		appMeta.Load(svcObj)

		// Load the Route resource
		routeObj := internal.GenerateObj(routeYaml)
		appMeta.Load(routeObj)

		processed, template, err := testInstance.Process(appMeta, routeObj)
		assert.NoError(t, err)
		assert.True(t, processed)

		var data string
		if rr, ok := template.(interface{ Data() string }); ok {
			data = rr.Data()
		} else {
			var buf bytes.Buffer
			err = template.Write(&buf)
			assert.NoError(t, err)
			data = buf.String()
		}
		// The target Service name in route template should be aligned.
		assert.Contains(t, data, `name: {{ include "my-app.fullname" . }}-api-service`)
	})
}
