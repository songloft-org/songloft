# Songloft Scripts Directory

This directory contains automation scripts for the Songloft backend, covering version management, build helpers, submodule syncing, and more. Multi-platform binary builds, Docker image packaging, and GitHub Release creation are all handled by [`.github/workflows/release.yml`](../.github/workflows/release.yml).

## 📋 Script List

| Script | Purpose |
|------|------|
| `bump-version.sh` | Bumps `VERSION` in `Makefile` and the Swagger `@version` in `main.go`, then commits + tags + pushes (once the tag is pushed, the release workflow takes over the release) |
| `submodule-update.sh` | Batch-syncs all git submodules to the latest main |
| `docker-entrypoint.sh` | Startup entrypoint inside the Docker image (not invoked directly) |
| `plugin-build.sh` | Builds a single JS plugin, outputting a `.jsplugin.zip` |
| `plugin-release.sh` | Uploads a `.jsplugin.zip` to the corresponding GitHub Release |
| `fetch-issues.mjs` / `sync-docs.mjs` | Documentation sync helpers |
| `test_tag.sh` | Manual smoke-test script for the `pkg/tag` command-line tools |

## 🚀 Release Workflow

```bash
# 1. Bump the version locally + create a tag + push
./scripts/bump-version.sh patch        # x.y.z → x.y.(z+1)
# or: make bump TYPE=patch
```

Once the tag is pushed, [`release.yml`](../.github/workflows/release.yml) automatically:

1. Builds lite + full editions of the binaries for 7 platforms (Linux amd64/arm64/armv7, macOS amd64/arm64, Windows amd64/arm64)
2. Builds and pushes multi-architecture Docker images (linux/amd64, linux/arm64, linux/arm/v7) to Docker Hub
3. Generates sha256 checksums
4. Creates a GitHub Release and uploads all artifacts
5. Uses [`requarks/changelog-action`](https://github.com/requarks/changelog-action) to generate a changelog from Conventional Commits, uses it as the Release Notes, and prepends the content to the top of `CHANGELOG.md`, committing it back to `main`

## 🔁 submodule-update.sh

Batch-syncs all submodules (`songloft-player` / `plugin-toolchain` / `pkg/tag` / `jsplugins-src/*` / `jsplugins`, etc.) to their respective main branches:

```bash
./scripts/submodule-update.sh
```

## 🔌 JS Plugin Build Scripts

| Script | Purpose |
|------|------|
| `plugin-build.sh <plugin-name>` | Enters `jsplugins-src/<plugin-name>/`, runs `pnpm install && pnpm run build`, and copies the artifacts into `jsplugins/` |
| `plugin-release.sh <plugin-name>` | Uploads the corresponding `.jsplugin.zip` to the GitHub Release of that plugin's submodule |

See `plugin-toolchain/README.md` for the detailed plugin development workflow.

## 🧪 test_tag.sh

A manual smoke test for the `cmd/tag`, `cmd/sum`, and `cmd/check` command-line tools provided by `pkg/tag`. Run `go install ./pkg/tag/cmd/...` before executing it.

## 📍 Repository Notes

- **Code & release repository**: https://github.com/songloft-org/songloft
- **Docker image**: [songloft/songloft](https://hub.docker.com/r/songloft/songloft) (keeps the original namespace)
