GOTFLATS ?=
SHELL = /bin/bash

define eachmod
	@echo '$(1)'
	@find . -type f -name go.mod -print0 | xargs -I '{}' -n1 -0 bash -c 'dir="$$(dirname {})" && echo "$${dir}" && cd "$${dir}" && $(1)'
endef

pre-commit:
	python -m pip install pre-commit --upgrade --user
	pre-commit install --install-hooks

tidy:
	$(call eachmod,go mod tidy)

mod-download:
	$(call eachmod,go mod download)

lint:
	$(call eachmod,GOGC=75 golangci-lint run --concurrency 32 --deadline 4m ./...)

build-all:
	$(call eachmod,go build -o /dev/null ./...)

docker:
	docker network create build-network || true
	docker run --rm --name goproxy --network build-network -d -v "${GOPATH}:/go" goproxy/goproxy || true
	docker build --network build-network --build-arg GOPROXY="http://goproxy:8081" -t ipfs/testground .
	docker rm -f goproxy

test:
	$(call eachmod,go test -p 1 -v $(GOTFLAGS) ./...)
