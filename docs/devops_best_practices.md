# DevOps Best Practices

## Container Images
- **Base images** – Prefer Red Hat certified images from `registry.access.redhat.com`. Example: `registry.access.redhat.com/ubi8/ubi`. Avoid using Docker Hub images directly; mirror them into the internal Quay registry (`tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br`).
- **Non‑root** – Build images that run as non‑root users (UID/GID ≥ 1000). Set `USER` in Dockerfile and validate with `docker run --user 1000 ...`.
- **SCC compliance** – Ensure the image does not request privileged capabilities, host networking, or root file system access unless the `anyuid` SCC is explicitly granted.

## CI/CD Pipelines (GitLab CI example)
```yaml
stages:
  - build
  - test
  - package
  - deploy

variables:
  IMAGE_REGISTRY: tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br
  IMAGE_NAME: $CI_PROJECT_PATH
  IMAGE_TAG: $CI_COMMIT_SHORT_SHA

build_image:
  stage: build
  image: registry.access.redhat.com/ubi8/ubi
  script:
    - podman build -t $IMAGE_REGISTRY/$IMAGE_NAME:$IMAGE_TAG .
    - podman push $IMAGE_REGISTRY/$IMAGE_NAME:$IMAGE_TAG
  only:
    - main

test_helmify:
  stage: test
  image: golang:1.22
  script:
    - go test ./...
    - helm lint ./examples/app || helm lint ./examples/operator

package_chart:
  stage: package
  image: registry.access.redhat.com/ubi8/ubi
  script:
    - go run ./cmd/helmify -f ./test_data sample-chart
    - tar -czf sample-chart.tar.gz sample-chart
  artifacts:
    paths:
      - sample-chart.tar.gz

deploy_to_openshift:
  stage: deploy
  image: registry.access.redhat.com/openshift4/ose-cli
  script:
    - oc login --token=$OPENSHIFT_TOKEN --server=$OPENSHIFT_API
    - oc project $PROJECT_NAME
    - helm upgrade --install helmify ./sample-chart --namespace $PROJECT_NAME
  only:
    - tags
```

## Security Scanning
- Run **Snyk** or **Trivy** on built images.
- Use **OpenShift Admission Controllers** to enforce SCC and non‑root policies.

## Repository Hygiene
- Keep `go.mod` tidy (`go mod tidy`).
- Run `golint`/`staticcheck` as part of CI.
- Clean up temporary files and test artifacts after each pipeline run.

These practices align with the corporate policy of containerizing applications before deploying to OpenShift.
