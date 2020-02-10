# Testground compositions

A testground composition is a TOML file that specifies, in a declarative manner,
all the information needed to run a testground job.

Compositions enable testers to build and schedule test runs comprising instances
built against different upstream dependencies. Compositions are defined in TOML
files, and amongst other things, they specify:

* the runner, builder, test plan, and test case.
* total instance count.
* groups participating in the run. For each group:
  - number of instances (as count or percentage).
  - upstream dependencies to override.
  - test parameters for that group.

## File format

The format for the TOML file is as follows:

```toml
## THIS IS AN EXAMPLE COMPOSITION
##
## It declares an execution of the dht/find-peers test case, with 100 instances:
##
##  * 10 of them are boostrappers, built against a concrete set of upstream dependency overrides.
##  * 45% are DHT clients, built against the latest releases of its upstream dependencies.
##  * 45% are DHT servers, using a pre-built image.
##
[metadata]
name    = "find-peers-01"
author  = "raulk"

[global]
plan    = "dht"
case    = "find-peers"
builder = "docker:go"
runner  = "local:docker"

total_instances = 100

[global.build_config]
go_version = "1.13"

[global.run_config]
keep_containers = true

[[groups]]
id = "bootstrappers"
instances = { count = 10 }

  [groups.build]
  dependencies = [
      { module = "github.com/libp2p/go-libp2p-kad-dht", version = "995fee9e5345fdd7c151a5fe871252262db4e788"},
      { module = "github.com/libp2p/go-libp2p", version = "76944c4fc848530530f6be36fb22b70431ca506c"},
  ]

  [groups.run]
  test_params = { random_walk = "true", n_bootstrap = "1" }

[[groups]]
id = "clients"
instances = { percentage = 0.45 }

  [groups.run]
  test_params = { server = "false", random_walk = "true", n_bootstrap = "1" }

[[groups]]
id = "servers"
instances = { percentage = 0.45 }

  [groups.run]
  artifact_path = "6bfc9d2c-7af1-4e2d-a8f1-f3a7cd4e1c1f"
  test_params = { server = "true", random_walk = "true", n_bootstrap = "1" }
```

## Building a composition

To build a composition, execute the following command:

```sh
$ ./testground build composition -f file.toml
```

To persist the resulting build artifact paths into the composition TOML file,
you can enable the `--write-artifacts` flag:

```sh
$ ./testground build composition -f file.toml --write-artifacts
```

Doing so will update the TOML file by setting the resulting build artifact paths
under the run.artifact_path fields of each group.

This is useful to bypass the cost of repetitive builds, such as when you want to
run the same composition mulitple times (such as when gathering multiple
observations). While builder-native caching alleviates this problem, e.g. Docker
layer caching, entirely bypassing the build step in these situations awards an
extra efficiency gain.

Note that docker image pushing is part of the `build` step currently, so you
cannot reuse a build artifact between different runners unless you make sure
the resulting build artifact is available to the runner.

## Running a composition

To run a composition, execute the following command:

```sh
$ ./testground run composition -f file.toml
```

Testground implicitly triggers a build for every group NOT carrying a
`run.artifact_path` value. All builds are performed in parallel.

If you'd like to save the resulting build artifacts in the TOML file, use the
`--write-artifacts` flag. Refer to the "Building a composition" section above
for more info.

If you'd like to _ignore_ all build artifacts present in the composition TOML,
you can use the `--ignore-artifacts` flag. Testground will trigger a fresh build
for each group (all of them in parallel).

Both options can be combined synergistically to refresh the build artifacts in
the composition TOML. The following command will trigger a fresh build for all
groups, and will persist all build artifacts in the composition TOML.

```sh
$ ./testground run composition -f file.toml --ignore-artifacts --write-artifacts
```
