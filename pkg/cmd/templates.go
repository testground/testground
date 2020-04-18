package cmd

const TEMPLATE_MAIN_GO = `package main

import "github.com/ipfs/testground/sdk/runtime"

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Hello, Testground!")
}
`

const TEMPLATE_GO_MOD = `module {{.}}

go 1.14

require github.com/ipfs/testground/sdk/runtime v0.4.0
`

const TEMPLATE_MANIFEST_TOML = `[defaults]
builder = "exec:go"
runner = "local:exec"

[builders."docker:go"]
enabled = true
go_version = "1.14"
module_path = "{{.}}"
exec_pkg = "."

[builders."exec:go"]
enabled = true
module_path = "{{.}}"

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
`
