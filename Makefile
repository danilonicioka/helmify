.PHONY: build lint container-build container-run clean

APP_NAME = helmify
IMAGE_TAG = latest

# Build the Go API
build:
	go build -buildvcs=false -o $(APP_NAME) ./cmd/helmify

# Run linter
lint:
	golangci-lint run ./...

# Build the container image locally
container-build: build
	podman build -t $(APP_NAME):$(IMAGE_TAG) .

# Run the container image locally
container-run: container-build
	podman run --rm -p 8080:8080 --name $(APP_NAME) $(APP_NAME):$(IMAGE_TAG)

# Clean up binaries
clean:
	rm -f $(APP_NAME)
