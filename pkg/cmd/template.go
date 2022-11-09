package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/testground/testground/pkg/api"

	"github.com/BurntSushi/toml"
)

type compositionData struct {
	Env map[string]string
}

func compileCompositionTemplate(path string, input *compositionData) (*bytes.Buffer, error) {
	templateDir := filepath.Dir(path)

	// Investigate: https://github.com/Masterminds/sprig
	// Investigate "missingkey=error"
	f := template.FuncMap{
		"pick": func(v map[string]interface{}, key string) map[string]interface{} {
			x := map[string]interface{}{key: v[key]}
			return x
		},
		"toml": func(v interface{}) (string, error) {
			var buf bytes.Buffer
			if err := toml.NewEncoder(&buf).Encode(v); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		"withEnv": func(value map[string]interface{}) map[string]interface{} {
			result := map[string]interface{}{}
			for k, v := range value {
				result[k] = v
			}
			result["Env"] = input.Env
			return result
		},
		"split": func(xs string) []string {
			return strings.Split(xs, ",")
		},
		"atoi": func(s string) (int, error) {
			return strconv.Atoi(s)
		},
		"load_resource": func(p string) (map[string]interface{}, error) {
			// NOTE: we do not worry about path that are leaving the template folders, or going through symlinks
			//		 because this is run on the client.
			fullPath := filepath.Join(templateDir, p)

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}

			var result map[string]interface{}
			if _, err := toml.Decode(string(data), &result); err != nil {
				return nil, fmt.Errorf("load_resource %s failed: %w", p, err)
			}

			return result, nil
		},
	}

	fdata, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse and run the composition as a template
	tpl, err := template.New("tpl").Funcs(f).Parse(string(fdata))
	if err != nil {
		return nil, err
	}
	buff := &bytes.Buffer{}
	err = tpl.Execute(buff, input)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func loadComposition(path string) (*api.Composition, error) {
	data := &compositionData{Env: map[string]string{}}

	// Build a map of environment variables
	for _, v := range os.Environ() {
		s := strings.SplitN(v, "=", 2)
		data.Env[s[0]] = s[1]
	}

	buff, err := compileCompositionTemplate(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to process composition template: %w", err)
	}

	comp := new(api.Composition)
	if _, err = toml.Decode(buff.String(), comp); err != nil {
		return nil, fmt.Errorf("failed to process composition file: %w", err)
	}

	comp, err = comp.PrepareFromUserInput()
	
	if err != nil {
		return nil, fmt.Errorf("failed to prepare composition: %w", err)
	}

	return comp, nil
}
