# Usage

## Setup

Ensure that you are running go 1.13 or later (for gomod support):

```bash
> go version
go version go1.13.1 darwin/amd64
```
Ensure you install Docker on your machine.

Then, onto getting the actual Testground code. Download the repo and build it:

```bash
> git clone https://github.com/ipfs/testground.git
> cd testground
> go build .
```

This command may take a couple of minutes to complete. If successful, it will end with no message.

Now, test that everything is installed correctly by running the following from within the source directory:

```bash
> ./testground
NAME:
   testground - A new cli application

USAGE:
   testground [global options] command [command options] [arguments...]

COMMANDS:
   run       (builds and) runs test case with name `<testplan>/<testcase>`. List test cases with `list` command
   list      list all test plans and test cases
   build     builds a test plan
   describe  describes a test plan or test case
   sidecar   runs the sidecar daemon
   daemon    start a long-running daemon process
   help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   -v          verbose output (equivalent to INFO log level)
   --vv        super verbose output (equivalent to DEBUG log level)
   --help, -h  show help
```

### How Testground guesses the source directory

In order to build test plans, Testground needs to know where its source directory is located. Testground can infer the path in the following circumstances:

1. When calling `testground` from PATH while situated in the source directory, or subdirectory thereof.
2. If the testground executable is situated in the source directory (such as when you do `go build .`), or a subdirectory thereof.

For special cases, supply the `TESTGROUND_SRCDIR` environment variable.

## Starting a Testground daemon

Testground has a daemon/client architecture:

* The daemon performs the heavy-lifting. It populates a catalogue of test plans,
  performs builds, and schedules runs, amongst other things.
    * The daemon is intended to run in a server setting, but can of course be
      run locally in a fashion similar to the IPFS daemon/client CLI.
    * The daemon exposes an HTTP API to receive client commands.
* The client is a lightweight CLI tool that sends commands to the daemon via its
  HTTP API.

Start the daemon with:

```bash
> testground -vv daemon
```

By default, the daemon will listen on endpoint `http://localhost:8042`. To
configure the listen address, refer to the `[daemon]` settings on the
[env-example.toml](../env-example.toml) file at the root of this repo.

The client CLI will also expect to find the daemon at `http://localhost:8042`.
To configure a custom endpoint address, refer to the `[client]` settings on the
[env-example.toml](../env-example.toml) file at the root of this repo.

## Pull the latest stable version of the Sidecar service (or build it locally from source)

```bash
docker pull ipfs/testground:latest
```

or


```bash
make docker-ipfs-testground
```

## Running test plans locally with Testground

To run a test plan locally, you can use the `testground run` command. Check what test plans are available in the `plans` folder

```bash
> testground list
attempting to guess testground source directory; for better control set ${TESTGROUND_SRCDIR}
successfully located testground source directory: /Users/imp/code/go-projects/src/github.com/ipfs/testground
warn: no .env.toml found; some components may not work
dht/find-peers
dht/find-providers
dht/provide-stress
dht/store-get-value
smlbench/simple-add
smlbench/simple-add-get
```

Before you run your first test plan, you need to build a Docker image that provides the "sidecar"

```
> make docker-ipfs-testground
```

This next command is your first test! It runs the `find-peers` test from the `dht`
test plan, using the builder (which sets up the environment + compilation) named
`docker:go` (which compiles Go inside Docker) and runs it using the runner
`local:docker` (which runs on your local machine).

```
> testground run single dht/find-peers \
    --builder=docker:go \
    --runner=local:docker \
    --instances=16
```

As of v0.1, you can also use compositions for a declarative method:
[docs/COMPOSITIONS.md](./COMPOSITIONS.md).

You should see a bunch of logs that describe the steps of the test, from:

* Setting up the container
* Compilation of the test case inside the container
* Starting the containers (total of 50 as 50 is the default number of nodes for this test)
* You will see the logs that describe each node connecting to the others and executing a kademlia find-peers action.

## Running a composition



## Running a test plan outside of the Testground orchestrator

You must have a Redis instance running locally. Install it for your runtime following instructions at https://redis.io/download.

Then run it locally with

```bash
> redis server
# ...
93801:M 03 Oct 2019 14:42:52.430 * Ready to accept connections
```

Then move into the folder that has the plan and test you want to run locally. Execute it by setting the `TEST_CASE` & `TEST_CASE_SEQ` environment variables:

```bash
> cd plans/dht
> TEST_CASE="lookup-peers" TEST_CASE_SEQ="0" go run main.go
# ... test output
```

## Running a test plan on Testground Cloud Infrastructure

### Getting your own backend running (create a cluster in AWS)

Follow the tutorial in the [infra folder](../infra/k8s/)

### Configure your local Testground environment

Testground automatically loads an `.env.toml` file at root of your source directory. It contains environment settings, such as:

* AWS secrets and settings.
* Builder and runner options. These are merged with values supplied via CLI, test plan manifests, and defaults.

You can initialize a new `.env.toml` file by copying the prototype [`env-example.toml`](env-example.toml) supplied in this repo to your testground source root. Refer to the comments in that example for explanations of usage.

### Running a test case in a AWS backend

1. Start a daemon locally
```bash
./testground --vv daemon
```

2. Use cluster:k8s runner and run a test plan, for example:
```bash
./testground --vv run single dht/find-peers \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --instances=16
```

## Creating a test case in Go

You can create test cases in any language. However, if you want to create one in Go, you can simply create a directory under `plans/` with the name of the test plan. We are going to use `test-plan`.

Inside, you should create a `main.go` file which will be the main entrypoint, as well as set up Go modules by creating a `go.mod` file. The main file should look something like:

```go
package main

import (
	test "github.com/ipfs/testground/plans/test-plan/test"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]runtime.TestCaseFn {
   "test-1": test.MyTest1,
   "test-2": test.MyTest2,
   // add any other tests you have in ./test
}

func main() {
	runtime.InvokeMap(testcases)
}
```

Each test, in this case, will be created under the subdirectory `./test`. For example, for `test.MyTest1`, it must be a function with the following signature:

```go
func MyTest1(runenv *runtime.RunEnv) error {
	// Your test...
}
```

Returning `nil` from a test case indicates that it completed successfully. If you return an error from a test case,
the runner will tear down the test, and the test outcome will be `"aborted"`. You can also halt test
execution by `panic`-ing, which causes the outcome to be recorded as `"crashed"`.

Inside `MyTest1` you can use any functions provided by [`RunEnv`](https://godoc.org/github.com/testground/sdk-go/runtime#RunEnv), 
such as [`runenv.Message`](https://godoc.org/github.com/testground/sdk-go/runtime#RunEnv.Message) and 
[`runenv.EmitMetric`](https://godoc.org/github.com/testground/sdk-go/runtime#RunEnv.EmitMetric).

To get custom parameters, passed via the flag `--test-param`, you can use the [param functions](https://godoc.org/github.com/testground/sdk-go/runtime).

### To get a simple string parameter

To use `myparam`, you should pass it to the test as `--test-param myparam="some value"`.

```go
func MyTest1(runenv *runtime.RunEnv) error {
   param, ok := runenv.StringParam("myparam")
   if !ok {
      // Param was not set
   }
}
```

### To get custom types

You can pass custom JSON like parameters, for example, if you want to send a map from strings to strings, you could do it like this:

```
testground run single test-plan/my-test-1 \
   --test-param myparam='{"key1": "value1", "key2": "value2"}'
   ...
```

And then you could get it like this:

```go
v := map[string]string{}
ok := runenv.JSONParam("myparam", &v)

fmt.Println(v)
// map[key1:value1 key2:value2]
```

### Networking & Sidecar

Where supported (all runners except the local:go runner), the "sidecar" service
is responsible for configuring the network for each test instance. At a
_minimum_, you should wait for the network to be initialized before proceeding
with your test.

```go
if err := sync.WaitNetworkInitialized(ctx context.Context, runenv, watcher); err != nil {
    return err // either panic, or make sure err propagates to the test case return value to abort test execution.
}
```

For more powerful network management (e.g., setting bandwidth limits and
latencies), see the
[sidecar](https://github.com/ipfs/testground/blob/master/docs/SIDECAR.md)
documentation.
