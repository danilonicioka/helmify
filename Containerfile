# Simplified Containerfile - uses pre-built artifacts from CI
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

LABEL org.opencontainers.image.source="https://github.com/danilonicioka/helmify"
LABEL org.opencontainers.image.description="Helmify Pro - Kubernetes manifest to Helm chart converter with Web UI"

WORKDIR /app

# The binary is built in the 'build-api' CI job
COPY helmify-api /usr/local/bin/helmify-api

# Ensure the binary is executable and handle OpenShift non-root user
RUN chmod +x /usr/local/bin/helmify-api && \
    chown 1001:0 /usr/local/bin/helmify-api

USER 1001
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/helmify-api"]
