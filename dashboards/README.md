# Dashboards on Testground

Testground has grafana!

## Get access to the grafana dashboard from your k8s cluster.

Prometheus and Grafana are running in the `monitoring` kubernetes namespace. Get access to
grafana locally and direct your web browser to `http://localhost:3000`

```
kubectl -n monitoring port-forward service/grafana 3000:3000

xdg-open http://localhost:3000 || open http://localhost:3000
```

Log in and change your password in the UI.

## Temporary dashboards
Any dashboard you create for yourself remains local to your copy of grafana. If you destroy your k8s
cluster, you will destroy the kubernetes dashboard as well. That's no good! Once you have your
dashboard just the way you like it, share it with the rest of the testground users by backing it up
into this dashboards directory!



## Saving a dashboard to git, for all to enjoy.
1. Tag your dashboard with the `testground` tag.
	- If you have additional useful tags, you can add those as well.
2. Run the dashboard code (in this directory)
```bash
$ export GRAFANA_API_KEY="XXX" 
$ go run main.go
```
or
```bash
$ go run main.go -apikey "XXX"
```


## Import dashboards to your grafana instance
(same as saving, but adding the -import flag)

```bash
$ export GRAFANA_API_KEY="XXX"
$ go run main.go -import
```
or
```bash
$ go run main.go -apikey "XXX" -import
```


# BUGS
Users need to create an API key through the web UI, which prevents this process from being easily
automated.
Maybe this will be less of a problem when this is run as a service.

I really *ought* to be able to upload these just by editing kubernetes configmaps -- this is of
course how the default prometheus operator dashboards are created.
