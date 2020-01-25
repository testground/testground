pre-commit:
	python -m pip install pre-commit --upgrade --user
	pre-commit install --install-hooks

docker-ipfs-testground:
	docker build -t ipfs/testground .

tidy:
	find . -type f -name go.mod -exec bash -c 'cd "$(dirname "{}")" && go mod tidy' \;
