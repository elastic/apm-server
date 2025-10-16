---
navigation_title: "Elastic APM"
mapped_pages:
  - https://www.elastic.co/guide/en/observability/current/apm-release-notes.html
  - https://www.elastic.co/guide/en/observability/master/release-notes-head.html
applies_to:
  stack: ga
---

# Elastic APM release notes

Review the changes, fixes, and more in each version of Elastic APM.

To check for security updates, go to [Security announcements for the Elastic stack](https://discuss.elastic.co/c/announcements/security-announcements/31).

% Release notes include only features, enhancements, and fixes. Add breaking changes, deprecations, and known issues to the applicable release notes sections.
% For each new version section, include the Elastic APM and Kibana changes.

% ## Next version [apm-next-release-notes]

% ### Features and enhancements [apm-next-features-enhancements]
% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))
% * Do not generate metrics or traces for pubsub bulk requests ([#18891](https://github.com/elastic/integrations/pull/18891))

% ### Fixes [apm-next-fixes]
% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))

## 9.2.0 [apm-9.2.0-release-notes]

```{applies_to}
stack: ga 9.2
```

### Features and enhancements [9-2-0-features-enhancements]

* [Failure store](https://www.elastic.co/docs/manage-data/data-store/data-streams/failure-store) enabled for all new datastreams. This is applied through [elasticsearch#131296](https://github.com/elastic/elasticsearch/pull/131296) and [elasticsearch#131395](https://github.com/elastic/elasticsearch/pull/131395)

## 9.1.5 [apm-9.1.5-release-notes]

### Fixes [apm-9.1.0-fixes]

* APM Integration on ECH or ECE uses recommended frozen values for infrastructure related configs. The fix is applied through [kibana#235306](https://github.com/elastic/kibana/pull/235306)

## 9.1.4 [apm-9.1.4-release-notes]

_No new features, enhancements, or fixes._

## 9.1.3 [apm-9.1.3-release-notes]

_No new features, enhancements, or fixes._

## 9.1.2 [apm-9.1.2-release-notes]

_No new features, enhancements, or fixes._

## 9.1.1 [apm-9.1.1-release-notes]

### Fixes [apm-9.1.1-fixes]

* Fix Fleet-managed apm-server hanging or crashing on config hot reload due to duplicate monitoring registry ([#17872](https://github.com/elastic/apm-server/pull/17872))

## 9.1.0 [apm-9.1.0-release-notes]

### Features and enhancements [apm-9.1.0-features-enhancements]

* Add config for tail-based sampling discard on write ([#13950](https://github.com/elastic/integrations/pull/13950))
* Add config for tail-based sampling TTL ([#16579](https://github.com/elastic/apm-server/pull/16579))

### Fixes [apm-9.1.0-fixes]

* Truncate string slice attributes in OTLP labels ([#434](https://github.com/elastic/apm-data/pull/434))
* Fix broken UI by explicitly enabling date detection for the `system.process.cpu.start_time` field ([#130466](https://github.com/elastic/elasticsearch/pull/130466))
* Use representative count for the `event.success_count` metric if available ([#119995](https://github.com/elastic/elasticsearch/pull/119995))
* Fix setting `event.dataset` to `data_stream.dataset` if `event.dataset` is empty, to have `event.dataset` in every `logs-*` data stream ([#129074](https://github.com/elastic/elasticsearch/pull/129074))
* Fix missing trusted root certificate authority in the docker image ([#16928](https://github.com/elastic/apm-server/pull/16928))

## 9.0.8 [apm-9.0.8-release-notes]

### Fixes [apm-9.0.8-fixes]

* APM Integration on ECH or ECE uses recommended frozen values for infrastructure related configs. The fix is applied through [kibana#235306](https://github.com/elastic/kibana/pull/235306).

## 9.0.7 [apm-9.0.7-release-notes]

_No new features, enhancements, or fixes._

## 9.0.6 [apm-9.0.6-release-notes]

_No new features, enhancements, or fixes._

## 9.0.5 [apm-9.0.5-release-notes]

_No new features, enhancements, or fixes._

## 9.0.4 [apm-9.0.4-release-notes]

### Fixes [apm-9.0.4-fixes]

* Tail-based sampling: Fix missing or infrequent monitoring metric `lsm_size` and `value_log_size` ([#17512](https://github.com/elastic/apm-server/pull/17512))
* Fix default tracer `http request sent to https endpoint` error when both self-instrumentation and TLS are enabled ([#17293](https://github.com/elastic/apm-server/pull/17293))

## 9.0.3 [apm-9.0.3-release-notes]

### Features and enhancements [apm-9.0.3-features-enhancements]

* Tail-based sampling: Optimize performance for instances with more CPU and memory ([#17254](https://github.com/elastic/apm-server/pull/17254))

### Fixes [apm-9.0.3-fixes]

* Tail-based sampling: Fix ever-increasing dynamic group count monitoring metric ([#17042](https://github.com/elastic/apm-server/pull/17042))
* Tail-based sampling: Fix incorrectly large pebble database `lsm_size` monitoring metric in edge cases which causes TBS to be bypassed ([#17149](https://github.com/elastic/apm-server/pull/17149))
* Tail-based sampling: Log pubsub errors at error or warn level ([#17135](https://github.com/elastic/apm-server/pull/17135))

## 9.0.2 [apm-9.0.2-release-notes]

### Fixes [apm-9.0.2-fixes]

* Fix missing trusted root certificate authority in the docker image ([#16928](https://github.com/elastic/apm-server/pull/16928))

## 9.0.1 [apm-9.0.1-release-notes]

### Fixes [apm-9.0.1-fixes]

* Tail-based sampling: ignore subscriber position read error and proceed as if file does not exist to avoid crash looping ([#16736](https://github.com/elastic/apm-server/pull/16736))

## 9.0.0 [9-0-0]

### Features and enhancements [9-0-0-features-enhancements]

* **Tail-based sampling**: Storage layer is rewritten to use Pebble database instead of BadgerDB. The new implementation offers a substantial throughput increase while consuming significantly less memory. Disk usage is significantly lower and more stable. See APM [Transaction sampling](docs-content://solutions/observability/apm/transaction-sampling.md) docs for benchmark details. ([#15235](https://github.com/elastic/apm-server/pull/15235))

### Fixes [9-0-0-fixes]

* Fix overflow in validation of `apm-server.agent.config.cache.expiration` on 32-bit architectures. ([#15216](https://github.com/elastic/apm-server/pull/15216))
* Change permissions of `apm-server.yml` in `tar.gz` artifacts to `0600`. ([#15627](https://github.com/elastic/apm-server/pull/15627))
