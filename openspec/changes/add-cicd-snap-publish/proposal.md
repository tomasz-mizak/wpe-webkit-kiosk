# Change: Add CI/CD pipeline for Snap Store publishing on tag

## Why
There is no automated build or publishing pipeline. Building and uploading the snap is a manual process which is error-prone and slows down releases. Automating this with GitHub Actions triggered by version tags ensures consistent, reproducible releases.

## What Changes
- Add a GitHub Actions workflow (`.github/workflows/publish-snap.yml`) that:
  - Triggers on tag push matching `v*` pattern
  - Builds the snap for amd64 using `snapcraft`
  - Publishes the built snap to the Snap Store **edge** channel
- Add documentation in README about the release process (tag → automatic publish)
- Requires a `SNAPCRAFT_STORE_CREDENTIALS` repository secret configured in GitHub

## Impact
- Affected specs: new `cicd-snap-publish` capability
- Affected code: new `.github/workflows/publish-snap.yml`, minor README update
- No changes to existing snap build logic or application code
- Prerequisites: Snap Store credentials token must be generated and stored as a GitHub secret
