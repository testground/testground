GOTFLATS ?=
SHELL = /bin/bash

define eachmod
	@echo '$(1)'
	@find . -type f -name go.mod -print0 | xargs -I '{}' -n1 -0 bash -c 'dir="$$(dirname {})" && echo "$${dir}" && cd "$${dir}" && $(1)'
endef

.PHONY: install goinstall sync-install pre-commit tidy mod-download lint build-all docker docker-sidecar docker-testground test-go test-integration test-integ-cluster-k8s test-integ-local-docker test-integ-local-exec kind-cluster

install: goinstall docker sync-install

goinstall:
	go install -ldflags "-X github.com/testground/testground/pkg/version.GitCommit=`git rev-list -1 HEAD`" .

sync-install:
	docker pull iptestground/sync-service:edge

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
	docker build --build-arg TG_VERSION=`git rev-list -1 HEAD` -t iptestground/sidecar:edge -f Dockerfile.sidecar .

docker-testground:
	docker build --build-arg TG_VERSION=`git rev-list -1 HEAD` -t iptestground/testground:edge -f Dockerfile.testground .

prepare-tests:
	testground plan import --from ./plans/placebo
	testground plan import --from ./plans/example

test-go: prepare-tests
	$(call eachmod,go test -p 1 -v $(GOTFLAGS) ./...)

test-integration: test-integ-cluster-k8s test-integ-local-exec test-integ-local-docker test-integ-examples

# Note that in these tests, we override the SYNC_SERVICE env var,
# so that testground connects to the instance exposed with `make kind-cluster`
test-integ-cluster-k8s: install kind-cluster
	SYNC_SERVICE_HOST=localhost ./integration_tests/01_k8s_kind_placebo_ok.sh
	SYNC_SERVICE_HOST=localhost ./integration_tests/02_k8s_kind_placebo_stall.sh

test-integ-local-exec: install
	./integration_tests/03_exec_go_placebo_ok.sh
	./integration_tests/20_exec_go_mod_rewrites.sh

test-integ-local-docker: install
	./integration_tests/04_docker_placebo_ok.sh
	./integration_tests/05_docker_placebo_stall.sh
	./integration_tests/06_docker_network_ping-pong.sh
	./integration_tests/07_docker_network_traffic-allowed.sh
	./integration_tests/08_docker_network_traffic-blocked.sh
	./integration_tests/09_docker_splitbrain_accept.sh
	./integration_tests/10_docker_splitbrain_reject.sh
	./integration_tests/11_docker_splitbrain_drop.sh
	./integration_tests/13_docker_builder_configuration.sh
	./integration_tests/13_02_docker_builder_configuration.sh
	./integration_tests/14_docker_silent_test_failure.sh
	./integration_tests/15_docker_mixed_builders_configuration.sh
	./integration_tests/15_02_docker_mixed_builders_and_custom_go_deps.sh
	./integration_tests/16_show_task_outcome_in_cli.sh
	./integration_tests/17_docker_benchmark_storm_ok.sh
	./integration_tests/18_block_runners.sh
	./integration_tests/19_limit_runs_per_branch.sh

test-integ-examples: install
	./integration_tests/example_01_rust.sh
	./integration_tests/example_02_js_pingpong.sh
	./integration_tests/example_03_generic_artifact.sh
	./integration_tests/example_04_browser.sh

kind-cluster:
	kind create cluster --wait 90s
	kubectl apply -f .circleci/pv.yaml
	kubectl apply -f .circleci/pvc.yaml
	kubectl label nodes kind-control-plane testground.node.role.plan=true
	kubectl label nodes kind-control-plane testground.node.role.infra=true
	kind load docker-image iptestground/sidecar:edge
	kubectl apply -f .circleci/sidecar.yaml

	kind load docker-image iptestground/sync-service:edge
	kubectl apply -f .circleci/sync-service.yaml
	kubectl expose deployment/testground-sync-service
	kubectl port-forward deployment/testground-sync-service 5050:5050 &
