## Context

The project produces a snap package (`wpe-webkit-kiosk`) that bundles WPE WebKit compiled from source. The build is resource-intensive (~1-2h on amd64). The snap is already registered in the Snap Store. Currently builds and uploads are done manually.

## Goals / Non-Goals

- Goals:
  - Automate snap building and publishing to edge channel on version tag push
  - Keep the pipeline simple and maintainable
  - Use free GitHub Actions infrastructure (ubuntu-latest runners)
- Non-Goals:
  - Multi-architecture builds (armhf, arm64) — only amd64 for now
  - PR build validation — not in scope, can be added later
  - Automatic promotion from edge to stable — manual via `snapcraft promote`

## Decisions

- **CI platform: GitHub Actions** — project is already on GitHub, no extra setup needed, free for public repos
- **Build method: `snapcraft` with LXD** — using the official `snapcraft-container` approach. The `snapcraft` GitHub Action (`snapcore/action-build`) handles LXD setup on the runner automatically
- **Publish method: `snapcore/action-publish`** — official action that uploads the `.snap` artifact and pushes to a specified channel
- **Trigger: tag push `v*`** — standard convention matching semver tags (e.g., `v2.50.5`). Does not require a GitHub Release to be created first; bare tag push is sufficient
- **Channel: edge** — safest for automated releases; manual `snapcraft promote` to move to candidate/stable
- **Credentials: `SNAPCRAFT_STORE_CREDENTIALS` secret** — scoped token with `package_upload` ACL for `wpe-webkit-kiosk` snap on edge channel only (principle of least privilege)

## Risks / Trade-offs

- **Long build times (~1-2h)** → GitHub Actions provides 6h timeout for public repos; sufficient but not fast. Mitigated by only running on tags (not on every push)
- **Runner disk space** → WebKit build requires significant disk space. `ubuntu-latest` runners have ~14GB free. May need a `free-disk-space` step to reclaim space from pre-installed tools
- **Snapcraft token expiry** → Tokens can expire; need to regenerate and update the secret periodically. Document this in README
- **No build cache** → Each build starts fresh. Snapcraft has some internal caching with LXD but it's per-run. Acceptable since builds only happen on release

## Open Questions

- None — all decisions confirmed via user input
