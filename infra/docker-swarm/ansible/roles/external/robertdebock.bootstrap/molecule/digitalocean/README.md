# Testing on Digitalocean

In order to test on Digitalocean, set the `DO_API_TOKEN`:

```
export DO_API_TOKEN=abcdefghijklmnopqrstuvwxyz0123456789
```

Run the tests:

```
molecule test --scenario-name digitalocean
```
