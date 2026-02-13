# Release Guide

This project publishes prebuilt `taskctl` binaries for macOS and Linux.

## Targets

- `darwin-arm64`
- `darwin-x86_64`
- `linux-x86_64`

## CI Workflows

- `.github/workflows/ci.yml`
  - Runs tests on Ubuntu.
  - Verifies release build on Ubuntu + macOS (x64/arm64).
- `.github/workflows/release.yml`
  - Trigger: tag push `v*`
  - Builds release binaries per target.
  - Packages assets and checksum files.
  - Uploads assets to GitHub Release.

## Local Packaging

```bash
./scripts/build-release.sh v0.1.0
```

Output in `dist/`:

- `taskctl-v0.1.0-<target>.tar.gz`
- `taskctl-v0.1.0-<target>.tar.gz.sha256`

## Release Steps (maintainer)

1. Ensure `taskctl/Cargo.toml` version is updated.
2. Push branch to `master` and confirm CI passes.
3. Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

4. Verify GitHub Release assets are generated for all targets.

## Install Steps (user)

```bash
./scripts/install.sh v0.1.0
```

Custom repo or install path:

```bash
REPO=biantaishabi2/Cli INSTALL_DIR=$HOME/.local/bin ./scripts/install.sh v0.1.0
```
