# Buildkite

This README provides an overview of the Buildkite pipeline used to automate the build and publish process for APM Server artifacts.

## APM Server Package pipeline

This is the Buildkite pipeline used to run the APM Server package generation and the DRA automation.

### Artifacts

The pipeline generates the following artifacts:

- **dependencies-APM-SERVER_VERSION-WORKFLOW.csv**: This CSV file contains a list of dependencies for the specific APM-Server version being built. It helps track build dependencies.

- **apm-server-APM-SERVER_VERSION-WORKFLOW-SUFFIX.EXT**: This file includes the APM Server binary.

### Triggering the Pipeline

The pipeline is triggered in the following scenarios:

- **Snapshot Builds**: A snapshot build is triggered when a pull request (PR) is merged into the 'main' branch or a version-specific branch.

- **Staging Builds**: A staging build is triggered when a PR is merged into a version-specific branch. Staging builds are typically used for a release build candidate.

After a successful build, the pipeline publishes the generated artifacts to the Google Cloud Storage (GCS) bucket named [elastic-artifacts-snapshot/apm-server](https://console.cloud.google.com/storage/browser/elastic-artifacts-snapshot/apm-server). You can access the published artifacts in this bucket.

### Pipeline Configuration

To view the pipeline and its configuration, click [here](https://buildkite.com/elastic/apm-server-package) or
go to the [catalog-info.yaml](../catalog-info.yaml) file.
