GOTFLATS ?=

define eachmod
	@echo '$(1)'
	@find . -type f -name go.mod -print0 | xargs -I '{}' -n1 -0 bash -c 'dir="$$(dirname {})" && echo "$${dir}" && cd "$${dir}" && $(1)'
endef

pre-commit:
	python -m pip install pre-commit --upgrade --user
	pre-commit install --install-hooks

docker-ipfs-testground:
	docker build -t ipfs/testground .

tidy:
	$(call eachmod,go mod tidy)

lint:
	$(call eachmod,golangci-lint run ./...)

test-build:
	$(call eachmod,go build -o /dev/null ./...)

test-quick:
	go list ./... | grep -v cmd | xargs go test

test-integration:
	docker build -t ipfs/testground .
	go test ./cmd/...
