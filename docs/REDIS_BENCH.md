## Redis benchmarks: initial AWS Docker Swarm cluster

> Author: @raulk
> Date: 2019-11-08

## Setup

Using the `redis:latest` image, which at the time was `de25a81a5a0b` (v5.0.6), I
started our `testground-redis` service on the Docker Swarm cluster:

```sh
$ docker service create --name "testground-redis" \
    --network control \
    --entrypoint redis-server \
    redis --notify-keyspace-events "\$szxK" --save \"\" --appendonly no
```

Then I started another service to run the benchmark. We are testing 1000
concurrent connections, with a payload size of 512 bytes.

```sh
$ docker service create --name "perf-redis" \
    --network control 
    redis redis-benchmark -c 1000 -h testground-redis -d 512
```

I made sure that Docker Swarm placed the containers in different machines; the
reason being that I want to model a realistic scenario that accounts for the
overhead of the overlay network.

As you observed below, Docker Swarm indeed placed the workloads in different
machines:

```sh
$ docker service ps testground-redis perf-redis
ID                  NAME                 IMAGE               NODE                DESIRED STATE       CURRENT STATE            ERROR               PORTS
15iv0b00oenr        perf-redis.1         redis:latest        ip-172-31-14-160    Running             Running 13 minutes ago
bpmxyw1s06jq        testground-redis.1   redis:latest        ip-172-31-6-112     Running             Running 47 minutes ago
```

I now tail the logs of the `perf-redis` service:

```sh
$ docker service logs -f perf-redis
```

The results are presented below.

## Results

(benchmark is still running; will update PR once it's finished.)