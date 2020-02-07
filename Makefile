GOTFLATS ?=

define eachmod
	@find . -type f -name go.mod -exec bash -c 'dir="$$(dirname {})" && cd "$${dir}" && echo "$${dir}: $(1)" && $(1)' \;
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
	docker build -t ipfs/testground .

test:
	$(call eachmod,go test -v $(GOTFLAGS) ./...)
