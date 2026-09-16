package k8smanifest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestTranslator_Translate_Stdin(t *testing.T) {
	yamlStr := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`

	conf := config.Config{}
	translator := New(conf, strings.NewReader(yamlStr))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := translator.Translate(ctx)
	assert.NoError(t, err)

	var payloads []string
	for p := range ch {
		if p.Object != nil {
			payloads = append(payloads, p.Object.GetName())
		}
	}

	assert.Len(t, payloads, 1)
	assert.Equal(t, "test-cm", payloads[0])
}
