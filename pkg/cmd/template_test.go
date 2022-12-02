package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	withResource        = "fixtures/templates/with-resource.toml"
	withResourceComplex = "fixtures/templates/with-resource-complex.toml"
	missingResource     = "fixtures/templates/missing-resource.toml"
	tomlAndWithEnv = "fixtures/templates/issue-1493-toml-and-with-env.toml"
)

func loadExpected(basePath string) (string, error) {
	data, err := os.ReadFile(basePath + ".expected")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestLoadCompositionWithResourcesGenerateTemplate(t *testing.T) {
	input := &compositionData{Env: map[string]string{}}
	buff, err := compileCompositionTemplate(withResource, input)
	if err != nil {
		t.Fatal(err)
	}

	str := buff.String()
	expected, err := loadExpected(withResource)

	require.Nil(t, err)
	require.Equal(t, expected, str)
}

func TestLoadCompositionWithResourcesComplexGenerateTemplate(t *testing.T) {
	input := &compositionData{Env: map[string]string{}}
	buff, err := compileCompositionTemplate(withResourceComplex, input)
	if err != nil {
		t.Fatal(err)
	}

	str := buff.String()
	expected, err := loadExpected(withResourceComplex)

	require.Nil(t, err)
	require.Equal(t, expected, str)
}

func TestLoadCompositionWithMissingResourcesFail(t *testing.T) {
	input := &compositionData{Env: map[string]string{}}
	buff, err := compileCompositionTemplate(missingResource, input)

	require.Error(t, err)
	require.Nil(t, buff)
}

func TestLoadCompositionWithTomlAndPickOperators(t *testing.T) {
	input := &compositionData{Env: map[string]string{
		"MyValue": "123",
	}}

	buff, err := compileCompositionTemplate(tomlAndWithEnv, input)
	if err != nil {
		t.Fatal(err)
	}

	str := buff.String()

	expected, err := loadExpected(tomlAndWithEnv)

	require.Nil(t, err)
	require.Equal(t, expected, str)
}
