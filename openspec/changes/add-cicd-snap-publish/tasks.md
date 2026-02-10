## 1. Setup

- [x] 1.1 Generate Snapcraft Store credentials token (`snapcraft export-login --snaps=wpe-webkit-kiosk --channels=edge --acls=package_upload`)
- [x] 1.2 Add the token as `SNAPCRAFT_STORE_CREDENTIALS` secret in the GitHub repository settings

## 2. Implementation

- [x] 2.1 Create `.github/workflows/publish-snap.yml` workflow file
  - Trigger: `push.tags: ['v*']`
  - Runner: `ubuntu-latest`
  - Steps: checkout → free disk space → build snap with `snapcraft` → upload artifact → publish to Snap Store edge channel
- [x] 2.2 Update README.md with release process section (how to create a tag, what happens automatically)

## 3. Validation

- [ ] 3.1 Push a test tag (e.g., `v0.0.1-test`) to verify the workflow triggers and builds correctly
- [ ] 3.2 Verify the snap appears in the Snap Store edge channel after successful build
- [ ] 3.3 Remove the test tag/release after verification
