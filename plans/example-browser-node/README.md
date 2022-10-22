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
in node, chrome and firefox:

```
integration_tests/example_05_browser_node.sh
```

Or in case you want, and already have a `testground daemon` running,
you can also run a single test case as follows:

```
testground run single \
    --plan example-browser-node \
    --testcase success \
    --instances 1 \
    --builder docker:generic \
    --runner local:docker \
    --tp browser=chromium \
    --wait
```
