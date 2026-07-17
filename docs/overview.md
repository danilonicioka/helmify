# Overview

Helmify is a CLI and Web API service that generates **Helm charts** from Kubernetes manifests. It supports plain manifests, Kustomize output, and OpenShift-specific resources (Routes, SCC, etc.). The tool aims to provide production‑ready, TJPA‑compliant charts with consistent labeling, non‑root containers, and deterministic rollouts.

## Why Helmify?
- Simplifies the transition from raw manifests to reusable Helm charts.
- Enforces best‑practice standards (labels, health probes, global config).
- Works in containerized environments and can be run as a microservice in OpenShift.

## High‑level Architecture
See the [Architecture](architecture.md) page for a detailed diagram and component breakdown.
