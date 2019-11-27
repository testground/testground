# Usage

We kindly ask you to read through the [SPEC](SPEC.md) first and give this project a run first in your local machine. It is a fast moving project at the moment, and it might require some tinkering and experimentation to compensate for the lack of documentation.

## Setup

Ensure that you are running go 1.13 or later (for gomod support):

```bash
> go version
go version go1.13.1 darwin/amd64
```

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
attempting to guess testground base directory; for better control set ${TESTGROUND_SRCDIR}
successfully located testground base directory: /Users/imp/code/go-projects/src/github.com/ipfs/testground
NAME:
   testground - A new cli application

   USAGE:
      testground [global options] command [command options] [arguments...]

   COMMANDS:
      run      (builds and) runs test case with name `testplan/testcase`
      list     list all test plans and test cases
      build    builds a test plan
      help, h  Shows a list of commands or help for one command

   GLOBAL OPTIONS:
      -v          verbose output (equivalent to INFO log level)
      --vv        super verbose output (equivalent to DEBUG log level)
     --help, -h  show help
```

#### How testground guesses the source directory

In order to build test plans, Testground needs to know where its source directory is located. Testground can infer the path in the following circumstances:

1. When calling testground from PATH while situated in the source directory, or subdirectory thereof.
2. If the testground executable is situated in the source directory (such as when you do `go build .`), or a subdirectory thereof.

For special cases, supply the `TESTGROUND_SRCDIR` environment variable.


### Running the tests locally with TestGround

To run a test locally, you can use the `testground run` command. Check what Test Plans are available in the `plans` folder

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

This next command is your first test! It runs the lookup-peers test from the DHT plan, using the builder (which sets up the environment + compilation) named docker:go (which compiles go inside docker) and runs it using the runner local:docker (which runs on your local machine).

```
> testground run dht/find-peers \
    --builder=docker:go \
    --runner=local:docker \
    --build-cfg bypass_cache=true
...
```

You should see a bunch of logs that describe the steps of the test, from:

* Setting up the container
* Compilation of the test case inside the container
* Starting the containers (total of 50 as 50 is the default number of nodes for this test)
* You will see the logs that describe each node connecting to the others and executing a kademlia find-peers action.

### Running a test outside of TestGround orchestrator

You must have a redis instance running locally. Install it for your runtime follow instruction at https://redis.io/download.

Then run it locally with

```bash
> redis server
# ...
93801:M 03 Oct 2019 14:42:52.430 * Ready to accept connections
```

Then move into the folder that has the plan and test you want to run locally. Execute it by sessting the TEST_CASE & TEST_CASE_SEQ env variables

```bash
> cd plans/dht
> TEST_CASE="lookup-peers" TEST_CASE_SEQ="0" go run main.go
# ... test output
```

### Running a Test Plan on the TestGround Cloud Infrastructure

#### Getting your own backend running (create a cluster in AWS)

Follow the tutorial in the [infra folder](../infra)

#### Link your local TestGround envinronment with your Docker Swarm Cluster running in AWS

Testground automatically loads an `.env.toml` file at root of your source directory. It contains environment settings, such as:

* AWS secrets and settings.
* Builder and runner options. These are merged with values supplied via CLI, test plan manifests, and defaults.

You can initialize a new `.env.toml` file by copying the prototype [`env-example.toml`](env-example.toml) supplied in this repo to your testground source root. Refer to the comments in that example for explanations of usage.

#### Running a test case in a AWS backend

Once you have done the step above, you will need to create an SSH tunnel to the AWS instanced created with the `terraform apply` call. To do so:

```bash
ssh -nNT -L 4545:/var/run/docker.sock ubuntu@<your.aws.thingy...compute.amazonaws.com>
```

Then, all you need to do is use cluster:swarm runner, example:

```bash
./testground -vv run dht/find-peers \
    --builder=docker:go \
    --runner=cluster:swarm \
    --build-cfg bypass_cache=true \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws
```

