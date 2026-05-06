.PHONY: build ui-build lint container-build clean

APP_NAME = helmify-pro
IMAGE_TAG = latest

# Build the Go API
build:
	go build -buildvcs=false -o $(APP_NAME) ./api

# Build the UI
ui-build:
	cd ui && npm install && npm run build

# Run linter
lint:
	golangci-lint run ./...

# Build the container image locally
container-build:
	podman build -t $(APP_NAME):$(IMAGE_TAG) .

# Clean up binaries
clean:
	rm -f $(APP_NAME)
	rm -rf ui/out
	rm -rf ui/.next
