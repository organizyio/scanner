ROOT := ../..
.PHONY: build test test-integration verify
build:
	python3 $(ROOT)/scripts/build-components.py wrapper
test:
	go test -mod=readonly ./...
test-integration:
	python3 $(ROOT)/scripts/build-components.py wrapper
	go test -mod=readonly -tags=integration ./tests/integration/... -count=1
verify:
	go mod verify
