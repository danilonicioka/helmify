.PHONY: build lint container-build clean

APP_NAME = helmify
IMAGE_TAG = latest

# Build the Go API
build:
	go build -buildvcs=false -o $(APP_NAME) ./cmd/helmify

# Run linter
lint:
	golangci-lint run ./...

# Build the container image locally
container-build:
	podman build -t $(APP_NAME):$(IMAGE_TAG) .

# Clean up binaries
clean:
	rm -f $(APP_NAME)

