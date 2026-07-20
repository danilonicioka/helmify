# Documentation Index

Welcome to the Helmify documentation hub. Below is the organized table of contents for building and deploying charts on Kubernetes and Red Hat OpenShift:

- **[Getting Started & Usage Guide](getting_started.md)**: Introduction to Helmify, running the tool (CLI & Web API), and a basic tutorial to create your first template.
- **[Architecture & Engine Overview](architecture.md)**: High-level repository layout, component pathways, and generation logic.
- **[TJPA Helm Chart Standard Specification](chart_standard_spec.md)**: Strict specification guide for values structures, naming schemes, component labels, and OpenShift multi-zone Routes.
- **[Templates Reference Guide](templates_guide.md)**: In-depth guide on the Go template engine, type checks, built-in variables, Sprig helpers, loops, logic, and debugging tools.
- **[Charts & Values Configuration](charts_and_values.md)**: Chart directory layout rules, values schemas (`values.schema.json`), global values, dependencies/subcharts, and local file access patterns.
- **[OpenShift & DevOps Best Practices](openshift_and_devops.md)**: Deployment restrictions under Security Context Constraints (SCC), RBAC setup (least privilege), CRDs management rules, and DevOps delivery guidelines.
- **[Reference & Conventions](reference.md)**: Naming conventions, YAML formatting rules, registries reference table, and Helm version compatibility maps.
- **[Contributing](contributing.md)**: Developer guidelines for extending Helmify parsers and wizards.
