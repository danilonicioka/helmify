FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL org.opencontainers.image.source="https://github.com/danilonicioka/helmify"
LABEL org.opencontainers.image.description="Helmify API - Kubernetes manifest to Helm chart converter"

WORKDIR /app
# Copy the pre-built binary from the CI build stage
COPY helmify-api /usr/local/bin/helmify-api

# Ensure the binary is executable
RUN chmod +x /usr/local/bin/helmify-api

# OpenShift requirement: run as non-root
USER 1001

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/helmify-api"]
