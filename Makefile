pre-commit:
	python -m pip install pre-commit --upgrade
	pre-commit install --install-hooks

docker-ipfs-testground:
	docker build -t ipfs/testground .
