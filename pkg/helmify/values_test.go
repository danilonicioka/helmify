package helmify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValues_Add(t *testing.T) {
	values := Values{}
	res, err := values.Add("abc", "a", "b")
	assert.NoError(t, err)
	assert.Equal(t, `{{ (index .Values "a").b | quote }}`, res)

	res, err = values.AddYaml("abc", 0, false, "a", "b")
	assert.NoError(t, err)
	assert.Equal(t, `{{ (index .Values "a").b | toYaml }}`, res)

	res, err = values.AddSecret(true, "a", "b")
	assert.NoError(t, err)
	assert.Equal(t, `{{ required "a.b is required" (index .Values "a").b | b64enc | quote }}`, res)
}
