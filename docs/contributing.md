# Contributing

We welcome contributions! Follow these steps to keep the project consistent and secure.

## Quick Checklist
- **Branch**: Create a feature branch from `main` (e.g., `feature/add‑my‑template`).
- **Code style**: Run `go fmt ./...` and `staticcheck ./...`.
- **Tests**: Add or update unit tests. Run `go test ./...` locally.
- **Lint docs**: Ensure Markdown files render without broken links (`markdownlint`). 
- **Update docs**: If you add a new template or change a behavior, update the relevant docs in `docs/` (e.g., `templates.md`, `helm_kustomize_openshift.md`).
- **Commit**: Squash commits into a single logical commit before opening a PR.
- **PR**: Reference the issue, describe the change, and ensure CI passes.

## PR Review Guidelines
- Verify that generated Helm charts pass `helm lint`.
- Check that container images remain non‑root and use approved registries.
- Confirm that any new labels follow the standard defined in `templates.md`.
- Make sure the documentation link in the root `README.md` points to the new page if applicable.

By following this flow we keep Helmify production‑ready, TJPA‑compliant, and easy to maintain.
