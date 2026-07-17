# Dependencies Best Practices

> **Warning**: This page has not yet been updated for Helm 4. Some of the content might be inaccurate or not applicable to Helm 4. For more information about the Helm 4 new features, improvements, and breaking changes, see **Helm 4 Overview**.

This section of the guide covers best practices for dependencies declared in `Chart.yaml`.

## Versions

- Where possible, use **version ranges** instead of pinning to an exact version. The suggested default is to use a patch‑level version match:

```yaml
version: ~1.2.3
```

- This will match version `1.2.3` and any patches to that release. In other words, `~1.2.3` is equivalent to `>= 1.2.3, < 1.3.0`.

- For the complete version‑matching syntax, please see the SemVer documentation.

## Prerelease Versions

- The above versioning constraints will **not** match pre‑release versions. For example, `version: ~1.2.3` will match `~1.2.4` but not `~1.2.3‑1`.
- To include a pre‑release while still matching patch‑level releases, use:

```yaml
version: ~1.2.3-0
```

## Repository URLs

- Prefer **HTTPS** URLs (`https://...`) and fall back to **HTTP** (`http://...`) when necessary.
- If the repository has been added to the repository index file, the repository **name** can be used as an alias of the URL. Use `alias:` or `@` followed by the repository name.
- **File URLs** (`file://...`) are considered a *special case* for charts assembled by a fixed deployment pipeline.
- When using **downloader plugins**, the URL scheme will be specific to the plugin. The chart’s user must have a plugin that supports the scheme installed to update or build the dependency.
- Helm cannot perform dependency‑management operations when the `repository` field is left blank. In that case Helm assumes the dependency is in a sub‑directory of the `charts/` folder with a name matching the `name` property of the dependency.

## Conditions and Tags

- Add **conditions** or **tags** to any dependencies that are optional. By default a condition is `true`.
- Preferred form of a condition:

```yaml
condition: somechart.enabled
```

  where `somechart` is the chart name of the dependency.
- When multiple sub‑charts (dependencies) together provide an optional or swappable feature, those charts should share the same tags.
- Example – if both `nginx` and `memcached` together provide performance optimisations for the main app, and are required to be present when the feature is enabled, they should both have a `tags` section like this:

```yaml
tags:
  - webaccelerator
```

- This allows a user to turn that feature on and off with a single tag.
