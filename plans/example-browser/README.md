# Example Browser Testplan

This plan serves two purposes:

- an integration test to proof the `docker:generic` builder + runner can be
  used to run complex docker-driven testplans by running and using a (headless) browser
  to evaluate your logic running on a webpage;
- a basic example to show how fairly easy it is to integrate this into your
  testground workflow using a `docker:generic` builder.

The latter is yet another way to showcase that the `docker:generic` builder allows
you to integrate whatever language and platform that you need.

## Usage

Within the root folder of this repository you can run the
integration test for this plan which will run all its test cases
on both chromium and firefox:

```
integration_tests/example_04_browser.sh
```

Or in case you want, and already have a `testground daemon` running,
you can also run a single test case as follows:

```
testground run single \
  --plan example-browser \
  --testcase success \
  --instances 1 \
  --builder docker:generic \
  --runner local:docker \
  --tp browser=chromium --wait
```
