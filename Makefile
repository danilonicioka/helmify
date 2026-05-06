.PHONY: build ui-build lint container-build clean

APP_NAME = helmify-pro
IMAGE_TAG = latest

# Build the Go API
build:
	@mkdir -p api/ui/out && touch api/ui/out/index.html
	go build -buildvcs=false -o $(APP_NAME) ./api

# Build the UI and copy to api/ for embedding
ui-build:
	cd ui && npm install && npm run build
	rm -rf api/ui/out && cp -r ui/out api/ui/out

# Run linter
lint:
	@mkdir -p api/ui/out && touch api/ui/out/index.html
	golangci-lint run ./...

# Build the container image locally
container-build:
	podman build -t $(APP_NAME):$(IMAGE_TAG) .

# Clean up binaries
clean:
	rm -f $(APP_NAME)
	rm -rf ui/out
	rm -rf ui/.next
	rm -rf api/ui/out
