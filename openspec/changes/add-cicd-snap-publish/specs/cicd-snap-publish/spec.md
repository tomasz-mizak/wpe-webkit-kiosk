## ADDED Requirements

### Requirement: Tag-Triggered Snap Build
The system SHALL build the snap package for amd64 architecture when a Git tag matching the `v*` pattern is pushed to the repository.

#### Scenario: Version tag triggers build
- **WHEN** a tag matching `v*` (e.g., `v2.50.5`) is pushed to the repository
- **THEN** a GitHub Actions workflow SHALL start that builds the snap using `snapcraft`

#### Scenario: Non-matching tag does not trigger build
- **WHEN** a tag not matching `v*` (e.g., `test-123`) is pushed
- **THEN** no build workflow SHALL be triggered

### Requirement: Automatic Snap Store Publishing
The system SHALL publish the successfully built snap to the Snap Store edge channel automatically.

#### Scenario: Successful build publishes to edge
- **WHEN** the snap build completes successfully
- **THEN** the resulting `.snap` file SHALL be uploaded to the Snap Store `edge` channel for the `wpe-webkit-kiosk` snap

#### Scenario: Failed build does not publish
- **WHEN** the snap build fails
- **THEN** no artifact SHALL be published to the Snap Store

### Requirement: Snap Build Artifact Retention
The system SHALL retain the built `.snap` file as a GitHub Actions artifact for download and manual inspection.

#### Scenario: Snap artifact is downloadable
- **WHEN** the build workflow completes (regardless of publish success)
- **THEN** the `.snap` file SHALL be available as a downloadable artifact in the GitHub Actions run
