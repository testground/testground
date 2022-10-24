# Example Browser and Node Testplan

This testplan serves as a more advanced and complete version
compared to [/plans/example-browsers](../example-browser/).

The difference here is that we want to showcase how one could,
while still using the `docker:generic` builder, have a testplan
which can be tested in a node environment (using `docker:node`)
as well as within a browser. This does require that the library
you wish to test can be used from both environments.

## Usage

Within the root folder of this repository you can run the
integration test for this plan which will run all its test cases
in node and chromium:

```
integration_tests/example_05_browser_node.sh
```

Or in case you want, and already have a `testground daemon` running,
you can also run a single test case as follows:

```
testground run single \
    --plan example-browser-node \
    --testcase output \
    --instances 1 \
    --builder docker:generic \
    --runner local:docker \
    --wait
```

## Remote Debugging

Using the `chrome://inspect` debugger tool,
as documented in <https://developer.chrome.com/docs/devtools/remote-debugging/local-server/>,
you can remotely debug this testplan.

This allows you to attach to the chrome browser which is running the plan.
Different with the [/plans/example-browsers](../example-browser/) plan
is that we only allow the chrome browser here, as to keep things simple here.

How to do it:

1. start the testplan
2. check what host port is bound to the exposed debug port
3. open `chrome://inspect` in your chrome browser on your host machine
4. configure the network targets to discover: `127.0.0.1:<your port>`
5. attach to the debugger using `inspect`

If you want you can now attach breakpoints to anywhere in the source code,
an a refresh of the page should allow you to break on it.

> TODO: support `debugger;` statements in the testplan,
> which should break and hang in the chrome debugger :|
