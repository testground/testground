GOTFLATS ?=
SHELL = /bin/bash

define eachmod
	@echo '$(1)'
	@find . -type f -name go.mod -print0 | xargs -I '{}' -n1 -0 bash -c 'dir="$$(dirname {})" && echo "$${dir}" && cd "$${dir}" && $(1)'
endef

.PHONY: install tidy mod-download lint build-all docker install test

install: goinstall docker

goinstall:
	go install .

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

docker: docker-testground docker-sidecar

docker-sidecar:
	docker build -t iptestground/sidecar:edge -f Dockerfile.sidecar .

docker-testground:
	docker build -t iptestground/testground:edge -f Dockerfile.testground .

test: install
	testground plan import --from ./plans/placebo
	testground plan import --from ./plans/example
	$(call eachmod,go test -p 1 -v $(GOTFLAGS) ./...)

test-integration:
	./integration_tests/01_k8s_kind_placebo_ok.sh
	./integration_tests/02_k8s_kind_placebo_stall.sh
	./integration_tests/03_exec_go_placebo_ok.sh
	./integration_tests/04_docker_placebo_ok.sh
	./integration_tests/05_docker_placebo_stall.sh

kind-cluster:
	kind create cluster --wait 90s
	kubectl apply -f .circleci/pv.yaml
	kubectl apply -f .circleci/pvc.yaml
	kubectl label nodes kind-control-plane testground.node.role.plan=true
	kubectl label nodes kind-control-plane testground.node.role.infra=true
