GOLANGCI_VERSION := 1.44.0
BINARY := k8s-promoter

.PHONY: build
build:
	@echo "==> Building ${BINARY}..."
	@go build  -o ${BINARY} cmd/k8s-promoter/main.go

.PHONY: test
test:
	@echo "==> Executing tests..."
	@bash -e -o pipefail -c 'go list ./... | xargs -n1 go test --timeout 30m -v -count 1'

.PHONY: lint
lint: 
	@echo "==> Installing golangci-lint..."
	@./scripts/install-golangci-lint.sh $(GOLANGCI_VERSION)
	@echo "==> Running golangci-lint..."
	@tools/golangci-lint run
