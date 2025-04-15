---
navigation_title: "Elastic APM"
mapped_pages:
  - https://www.elastic.co/guide/en/observability/current/apm-release-notes.html
  - https://www.elastic.co/guide/en/observability/master/release-notes-head.html
---

# Elastic APM release notes

Review the changes, fixes, and more in each version of Elastic APM.

To check for security updates, go to [Security announcements for the Elastic stack](https://discuss.elastic.co/c/announcements/security-announcements/31).

% Release notes include only features, enhancements, and fixes. Add breaking changes, deprecations, and known issues to the applicable release notes sections.
% For each new version section, include the Elastic APM and Kibana changes.

% ## version.next [elastic-apm-next-release-notes]

% ### Features and enhancements [elastic-apm-next-features-enhancements]

% ### Fixes [elastic-apm-next-fixes]

## 9.0.0 [9-0-0]

### Features and enhancements [9-0-0-features-enhancements]

* **Tail-based sampling**: Storage layer is rewritten to use Pebble database instead of BadgerDB. The new implementation offers a substantial throughput increase while consuming significantly less memory. Disk usage is significantly lower and more stable. See APM [Transaction sampling](docs-content://solutions/observability/apm/transaction-sampling.md) docs for benchmark details. ([#15235](https://github.com/elastic/apm-server/pull/15235))

### Fixes [9-0-0-fixes]

* Fix overflow in validation of `apm-server.agent.config.cache.expiration` on 32-bit architectures. ([#15216](https://github.com/elastic/apm-server/pull/15216))
* Change permissions of `apm-server.yml` in `tar.gz` artifacts to `0600`. ([#15627](https://github.com/elastic/apm-server/pull/15627))
