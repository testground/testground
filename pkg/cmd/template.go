package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/testground/testground/pkg/api"

	"github.com/BurntSushi/toml"
)

type compositionData struct {
	Env map[string]string
}

func loadComposition(file string) (*api.Composition, error) {
	fdata, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	data := &compositionData{Env: map[string]string{}}

	// Build a map of environment variables
	for _, v := range os.Environ() {
		s := strings.SplitN(v, "=", 2)
		data.Env[s[0]] = s[1]
	}

	f := template.FuncMap{
		"split": func(xs string) []string {
			return strings.Split(xs, ",")
		},
	}

	// Parse and run the composition as a template
	tpl, err := template.New("tpl").Funcs(f).Parse(string(fdata))
	if err != nil {
		return nil, err
	}
	buff := &bytes.Buffer{}
	err = tpl.Execute(buff, data)
	if err != nil {
		return nil, err
	}

	comp := new(api.Composition)

	if _, err = toml.Decode(buff.String(), comp); err != nil {
		return nil, fmt.Errorf("failed to process composition file: %w", err)
	}

	return comp, nil
}
