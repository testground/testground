GOTFLATS ?=
SHELL = /bin/bash

define eachmod
	@echo '$(1)'
	@find . -type f -name go.mod -print0 | xargs -I '{}' -n1 -0 bash -c 'dir="$$(dirname {})" && echo "$${dir}" && cd "$${dir}" && $(1)'
endef

all: rm-docker-container-deps build-sidecar-image install

rm-docker-container-deps:
	docker rm -f testground-sidecar || true
	docker rm -f testground-redis || true
	docker rm -f testground-goproxy || true

install:
	go install ./...

pre-commit:
	python -m pip install pre-commit --upgrade --user
	pre-commit install --install-hooks

build-sidecar-image:
	docker build -t ipfs/testground .

tidy:
	$(call eachmod,go mod tidy)

lint:
	$(call eachmod,GOGC=75 golangci-lint run --build-tags balsam --concurrency 32 --deadline 4m ./...)

test-build:
	$(call eachmod,go build -tags balsam -o /dev/null ./...)
	docker build -t ipfs/testground .

test:
	$(call eachmod,go test -tags balsam -p 1 -v $(GOTFLAGS) ./...)
