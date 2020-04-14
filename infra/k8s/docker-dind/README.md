# Docker images for `docker-dind` used in Testground Kubernetes setup

We used Docker in Docker to build images by the Testground daemon pod in Kubernetes

```

pushd buster
docker build -t nonsens3/debian:buster .

pushd dind
docker build -t nonsens3/docker-dind:buster .

popd
popd
```

## References

Original Dockerfiles are from https://github.com/vicamo/docker-dind
