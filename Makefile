BIN_DIR ?= bin
LDFLAGS := -s -w
GOFLAGS = -gcflags "all=-trimpath=$(PWD)" -asmflags "all=-trimpath=$(PWD)"
GO_BUILD_ENV_VARS := GO111MODULE=on CGO_ENABLED=0

q3: gen
	@$(GO_BUILD_ENV_VARS) go build -o $(BIN_DIR)/q3 $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/q3

gen: ## Generate and embed templates
	@go run tools/genstatic.go public public

VERSION ?= $(shell semver-gen generate -l 2>/dev/null | awk '{print $$2}' || echo "latest")
IMAGE_BASE ?= ghcr.io/lukaszraczylo/q3
IMAGE   ?= $(IMAGE_BASE):$(VERSION)

.PHONY: build
build:
	@docker build . --force-rm --build-arg GOPROXY --build-arg GOSUMDB -t $(IMAGE)

.PHONY: buildx
buildx:
	@docker buildx build . --platform=linux/amd64,linux/arm64 --progress=plain -t $(IMAGE) -t $(IMAGE_BASE):latest --push

.PHONY: test
test:
	@go test -v ./internal/...
