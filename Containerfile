# Stage 1: Build the UI
FROM registry.access.redhat.com/ubi8/nodejs-20:latest AS ui-builder
WORKDIR /opt/app-root/src
COPY ui/package*.json ./
RUN npm install
COPY ui/ ./
RUN npm run build

# Stage 2: Build the Go API
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS api-builder
WORKDIR /opt/app-root/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy the built UI from the previous stage
COPY --from=ui-builder /opt/app-root/src/out ./ui/out
RUN CGO_ENABLED=0 go build -o helmify-api ./cmd/helmify-api

# Stage 3: Final Production Image
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
LABEL org.opencontainers.image.source="https://github.com/danilonicioka/helmify"
LABEL org.opencontainers.image.description="Helmify Pro - Kubernetes manifest to Helm chart converter with Web UI"

WORKDIR /app
COPY --from=api-builder /opt/app-root/src/helmify-api /usr/local/bin/helmify-api

# Ensure the binary is executable and handle OpenShift non-root user
RUN chmod +x /usr/local/bin/helmify-api && \
    chown 1001:0 /usr/local/bin/helmify-api

USER 1001
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/helmify-api"]
