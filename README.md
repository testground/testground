# InterPlanetary TestGround

![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg?style=flat-square)

> ⚠️ **Heavy WIP.** beware of the Dragons 🐉..

> **This repository is incubating the InterPlanetary Testground. 🐣**

## Description

You may have noticed a few test efforts with similar names underway! Testing at scale is a hard problem. We are indeed exploring and experimenting a lot, until we land on an end-to-end solution that works for us.

-  Interplanetary Testbed (IPTB): https://github.com/ipfs/iptb
  - a simple utility to manage local clusters/aggregates of IPFS instances.
- libp2p testlab: https://github.com/libp2p/testlab
  - a Nomad deployer for libp2p nodes with a DSL for test scenarios.
- And others such as https://github.com/ipfs/interop and https://github.com/ipfs/benchmarks

The Interplanetary Test Ground aims to leverage the learnings and tooling resulting from those efforts to provide a scalable runtime environment for the execution of various types of tests and benchmarks, written in different languages, by different teams, targeting a specific commit of IPFS and/or libp2p, and quantifying its characteristics in terms of performance, resource and network utilisation, stability, interoperability, etc., when compared to other commits.

The Interplanetary Test Ground aims to be tightly integrated with the software engineering practices and tooling the IPFS and libp2p teams rely on.

## Team

The current TestGround Team is composed of:

- @raulk - Lead Architect, Engineer, Developer
- @daviddias - Engineer, Developer, acting as interim PM for the project
- @jimpick - Engineer, Developer, Infrastructure Lead
- you! Yes, you can contribute as well, however, do understand that this is a brand new and fast moving project and so contributing might require extra time to onboard

We run a Weekly Sync at 4pm Tuesdays on [Zoom Room](https://protocol.zoom.us/j/299213319), notes are taken at [hackmd.io test-ground-weekly/edit](https://hackmd.io/@daviddias/test-ground-weekly/edit?both) and stored at [meeting-notes](https://github.com/ipfs/testground/tree/master/_meeting-notes). This weekly is listed on the [IPFS Community Calendar](https://github.com/ipfs/community#community-calendar). Recordings can be found [here](https://drive.google.com/open?id=1VL57t9ZOtk5Yw-cQoG7TtKaf3agDsrLc)(currently only available to the team).

We track our work Kanban style in a [Zenhub board](https://app.zenhub.com/workspaces/test-ground-5db6a5bf7ca61c00014e2961/board?repos=197244214) (plus, if you want to give your browser super powers, get the [Zenhub extension](https://www.zenhub.com/extension)). Notes on using the Kanban:
- The multiple stages are:
  - **Inbox** - New issues or PRs that haven't been evaluated yet
  - **Icebox** - Low priority, un-prioritized Issues that are not immediate priorities.
  - **Blocked** - Issues that are blocked or discussion threads that are not currently active
  - **Ready** - Upcoming Issues that are immediate priorities. Issues here should be prioritized top-to-bottom in the pipeline.
  - **In Progress** - Issues that someone is already tackling. Contributors should focus on a few things rather than many at once.
  - **Review/QA** - Issues open to the team for review and testing. Code is ready to be deployed pending feedback.
  - **OKR** - This column is just a location for the OKR cards to live until all the work under them is complete.
  - **Closed/Done** - Issues are automatically moved here when the issue is closed or the PR merged. Means that the work of the issue has been complete.
- We label issues using the following guidelines:
  - `difficulty:{easy, moderate, hard}` - This is an instinctive measure give by the project lead, project maintainer and/or architect.. It is a subjective best guess, however the current golden rule is that an issue with difficulty:easy should not require more than a morning (3~4 hours) to do and it should not require having to mess with multiple modules to complete. Issues with difficulty moderate or hard might require some discussion around the problem or even request that another team (i.e go-ipfs) makes some changes. The length of moderate or hard issue might be a day to ad-aeternum.
  - `priority (P0, P1, P2, P3, P4)` - P0 is the most important while P4 is the least.
  - `good first issue` - Issues perfect for new contributors. They will have the information necessary or the pointers for a new contributor to figure out what is required. These issues are never blocked on some other issue be done first.
  - `help wanted` - A label to flag that the owner of the issue is looking for support to get this issue done.
-   `blocked` - Work can't progress until a dependency of the issue is resolved.
- Responsibilities:
  - Project Maintainer and/or Project Architect - Review issues on Inbox, break them down if necessary, move them into Ready when it is the right time. Also, label issues with priority and difficulty.
  - Contributors move issues between the Ready, In Progress and Review/QA Colums. Use help wanted and blocked labels in case they want to flag that work.

## Architecture

Refer to the [specification](docs/SPEC.md) document.

## Repo Structure

```
├── README.md                       # This file
├── main.go                         # TestGround entrypoint file
├── cmd                             # TestGround CLI comamnds
│   ├── all.go
│   ├── ...
├── manifests                       # Manifests for each test Plan. These exist independent from plans to enable plans to live elsewhere
│   ├── dht.toml
│   └── smlbench.toml
├── plans                           # The Test Plan. Includes Image to be run, Assertions and more
│   ├── dht
│   └── smlbench
├── sdk                             # SDK available to each test plan
│   ├── runtime
│   └── ...
├── docs                            # Documentation of the project
│   ├── SPEC.md
│   ├── ...
├── pkg                             # Internals to TestGround
│   ├── api
│   ├── ...
└── tools                           # ??
    └── src_generate.go
```

## Contributing & Testing

We kindly ask you to read through the SPEC first and give this project a run first in your local machine. It is a fast moving project at the moment and it might require some tinkering and experimentation to compesate the lack of documentation.

### Setup

Ensure that you are running go 1.13 or later (for gomod support):

```sh
> go version
go version go1.13.1 darwin/amd64
```

Then, onto getting the actual Testground code. Download the repo and build it:

```sh
> git clone https://github.com/ipfs/testground.git
> cd testground
> go build .
```

This command may take a couple of minutes to complete. If successful, it will end with no message.

Now, test that everything is installed correctly by running the following from within the source directory:

```sh
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

```
> testground list
attempting to guess testground base directory; for better control set ${TESTGROUND_SRCDIR}
successfully located testground base directory: /Users/imp/code/go-projects/src/github.com/ipfs/testground


dht/lookup-peers
dht/lookup-providers
dht/store-get-value
smlbench/lookup-peers
smlbench/lookup-providers
smlbench/store-get-value
```

This next command is your first test! It runs the lookup-peers test from the DHT plan, using the builder (which sets up the environment + compilation) named docker:go (which compiles go inside docker) and runs it using the runner local:docker (which runs on your local machine).

```
> testground -vv run dht/lookup-peers --builder=docker:go --runner=local:docker --build-cfg bypass_cache=true
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

```
> redis server
# ...
93801:M 03 Oct 2019 14:42:52.430 * Ready to accept connections
```

Then move into the folder that has the plan and test you want to run locally. Execute it by sessting the TEST_CASE & TEST_CASE_SEQ env variables

```
> cd plans/dht
> TEST_CASE="lookup-peers" TEST_CASE_SEQ="0" go run main.go
# ... test output
```

### Running a Test Plan on the TestGround Cloud Infrastructure

#### Setting an environment file

Testground automatically loads an `.env.toml` file at root of your source directory. It contains environment settings, such as:

* AWS secrets and settings.
* Builder and runner options. These are merged with values supplied via CLI, test plan manifests, and defaults.

You can initialize a new `.env.toml` file by copying the prototype [`env-example.toml`](env-example.toml) supplied in this repo to your testground source root. Refer to the comments in that example for explanations of usage.

#### Running in the Cloud

`To be Written once such infrastructure exists..soon™`

## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

You can contact us on the freenode #ipfs-dev channel or attend one of our [weekly calls](https://github.com/ipfs/team-mgmt/issues/674).

## License

Dual-licensed: [MIT](./LICENSE-MIT), [Apache Software License v2](./LICENSE-APACHE), by way of the [Permissive License Stack](https://protocol.ai/blog/announcing-the-permissive-license-stack/).
