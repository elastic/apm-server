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

## Next version [elastic-apm-next-release-notes]

### Features and enhancements [elastic-apm-next-features-enhancements]
% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))

### Fixes [elastic-apm-next-fixes]
% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))

## 9.1.0 [9-1-0]

### Features and enhancements [9-1-0-features-enhancements]

_No new features or enhancements_ 

### Fixes [9-1-0-fixes]

_No new fixes_ 

## 9.0.3 [elastic-apm-9.0.3-release-notes]

### Features and enhancements [elastic-apm-9.0.3-features-enhancements]

* Tail-based sampling: Optimize performance for instances with more CPU and memory ([#17254](https://github.com/elastic/apm-server/pull/17254))

### Fixes [elastic-apm-9.0.3-fixes]

* Tail-based sampling: Fix ever-increasing dynamic group count monitoring metrics ([#17042](https://github.com/elastic/apm-server/pull/17042))
* Tail-based sampling: Fix incorrectly large pebble database `lsm_size` monitoring metric for ([#17149](https://github.com/elastic/apm-server/pull/17149))
* Tail-based sampling: Log pubsub errors at error or warn level ([#17135](https://github.com/elastic/apm-server/pull/17135))
* Tail-based sampling: Fix infrequent storage monitoring metrics ([#17154](https://github.com/elastic/apm-server/pull/17154))

## 9.0.2 [elastic-apm-9.0.2-release-notes]

### Fixes [elastic-apm-9.0.2-fixes]

* Fix missing trusted root certificate authority in the docker image ([#16928](https://github.com/elastic/apm-server/pull/16928))

## 9.0.1 [elastic-apm-9.0.1-release-notes]

### Fixes [elastic-apm-9.0.1-fixes]

* Tail-based sampling: ignore subscriber position read error and proceed as if file does not exist to avoid crash looping ([#16736](https://github.com/elastic/apm-server/pull/16736))

## 9.0.0 [9-0-0]

### Features and enhancements [9-0-0-features-enhancements]

* **Tail-based sampling**: Storage layer is rewritten to use Pebble database instead of BadgerDB. The new implementation offers a substantial throughput increase while consuming significantly less memory. Disk usage is significantly lower and more stable. See APM [Transaction sampling](docs-content://solutions/observability/apm/transaction-sampling.md) docs for benchmark details. ([#15235](https://github.com/elastic/apm-server/pull/15235))

### Fixes [9-0-0-fixes]

* Fix overflow in validation of `apm-server.agent.config.cache.expiration` on 32-bit architectures. ([#15216](https://github.com/elastic/apm-server/pull/15216))
* Change permissions of `apm-server.yml` in `tar.gz` artifacts to `0600`. ([#15627](https://github.com/elastic/apm-server/pull/15627))
