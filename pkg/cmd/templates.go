package cmd

type templated struct {
	Filename string
	Template string
}

type templateSet []templated

type templateVars struct {
	Name   string
	Module string
}

// GetTemplateSet returns a set of Go templates with their filenames which would be apprpriate
// for the garget language.
func GetTemplateSet(name string) templateSet {
	return map[string]templateSet{
		"go": go_templates,
	}[name]
}

// go_templates, is a templateSet for the go language.
// These templates are those used in the gitbook documentation, and hopefully are just useful enough
// to people started.
var go_templates = templateSet{
	templated{
		Filename: "main.go",
		Template: `// Welcome, testground plan writer!
// If you are seeing this for the first time, check out our documentation!
// https://app.gitbook.com/@protocol-labs/s/testground/

package main

import "github.com/ipfs/testground/sdk/runtime"

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Hello, Testground!")
	return nil
}
`},
	templated{
		Filename: "go.mod",
		Template: `module {{.Module}}

go 1.14

require github.com/ipfs/testground/sdk/runtime v0.4.0
`},
	templated{
		Filename: "manifest.toml",
		Template: `name = "{{.Name}}"
[defaults]
builder = "exec:go"
runner = "local:exec"

[builders."docker:go"]
enabled = true
go_version = "1.14"
module_path = "{{.Module}}"
exec_pkg = "."

[builders."exec:go"]
enabled = true
module_path = "{{.Module}}"

[runners."local:docker"]
enabled = true

[runners."local:exec"]
enabled = true

[runners."cluster:k8s"]
enabled = true

[[testcases]]
name= "quickstart"
instances = { min = 1, max = 5, default = 1 }

# Add more testcases here...
# [[testcases]]
# name = "another"
# instances = { min = 1, max = 1, default = 1 }
#   [testcase.params]
#   param1 = { type = "int", desc = "an integer", unit = "units", default = 3 }
`},
}
