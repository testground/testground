# Remote Runner Plan

This is an example of a Testground plan to build an application with Nix and run
the application on different machines. It's a prototype, so expect this to
change and to be better supported in the future.

At a high level, this works by:
1. Using [Nix] to build the application for each machine's architecture
   (e.g. One node may be `x86_64-linux` and another may be `aarch64-darwin`).
   You may need to set up [remote
   builders](https://nixos.org/manual/nix/stable/advanced-topics/distributed-builds.html)
   to properly cross-build the application.
1. Using a `local:exec` runner to run a shell script that copies the built
   application (and any dependencies) to each remote machine.
1. The shell script will call the `choose-server` helper tool to choose which
   server the instance will run under. It does this with `.MustSignalEntry` and
   uses the seq number to get the remote information from the `remote-runners` param.
1. Creating a temporary directory for the outputs on the remote machine.
1. Having the shell script run the application over `ssh` and port forward the
   testground services running locally.
1. When the application finishes, we copy the outputs from the remote machine to
   our local machine.

# Configuration

The remote servers are configured with `remote-runners` param. See the
`manifest.toml` file for an example of using `localhost` as the first instance
and a remote machine as the second instance. The `choose-server` helper tool
enrolls each instance and gets a sequence number. It uses this sequence number
to pick which machine this application will run under by indexing into the
`remote-runners` param. If the name of the remote runner is `localhost` it will
run locally. If the name is something else it'll run the application on that
host.  Note that you need to define the remote machine's architecture so that
the shell script can copy the right application binary over (unless the name is
localhost, in which case arch can be inferred).

# Usage

```bash
testground run single \
    --plan=remote-runner \
    --testcase="server-client" \
    --builder "nix:generic" \
    --runner="local:exec" \
    --instances=2 \
    --wait \
    --test-param remote-runners='[{"name":"localhost"},{"arch":"x86_64-linux","name":"<SSH_TARGET_NAME>"}]' \
    --test-param server_ip=<LOCALOST_IP>
```

For example, when I run this on my home network I run:

```bash
testground run single \
    --plan=remote-runner \
    --testcase="server-client" \
    --builder "nix:generic" \
    --runner="local:exec" \
    --instances=2 \
    --wait \
    --test-param remote-runners='[{"name":"localhost"},{"arch":"x86_64-linux","name":"dex"}]' \
    --test-param server_ip=192.168.195.204
```



[Nix]: (https://nixos.org/)
