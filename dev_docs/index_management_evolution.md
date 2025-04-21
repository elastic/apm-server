## Index Management Evolution

Over multiple releases, APM Server has undergone several changes in how it manages indices. These changes have been implemented incrementally, making it challenging for users to track the evolution and understand the current state of index management. Index management is an aggregated term used for the following components:

1. Index Templates
2. Component Templates
3. Ingest Pipelines
4. ILM Policies
5. Data Streams

The primary goal of this document is to create detailed timeline that captures:

- Sequence of changes made to index management across different releases.
- Retionale behind these changes.
- Impact on users and their configurations, ie bugs that was introduced and fixed.

## Summary

1. [elastic/apm-server](https://github.com/elastic/apm-server)
    - Initially, APM Server managed its own index templates and ILM policies.
    - With Version 8.0, index management shifted to Fleet, removing them from APM Server.
    - By Version 8.15, APM Server began relying on the ES `apm-data` plugin, further decoupling index management from the server itself.
    - Leveraging the `apm-data` plugin:
        - Simplifies setup for user of APM Server binary.
        - Replaces the need for installing any Fleet integration package for Elastic APM.
        - Removes the possibility of index templates being missing on startup.
2. [elastic/integrations](https://github.com/elastic/integrations)
    - Previously, the APM Integrations package was responsible for index management in Fleet for APM Server.
    - Transitioning to the `apm-data` plugin required removing templates created by the integration to prevent conflict, as these templates has higher priority and cloud override those from the plugin.
3. [elastic/elasticsearch](https://github.com/elastic/elasticsearch)
    - The introduction of the `apm-data` plugin in ES "moved" index management one abstraction layer closer to the actual data layer.
    - Resulting in a more streamlined setup and reducing the dependency on external integrations for a standalone APM Server.

## 8.15.0 - (Release: Aug 2, 2024)

- **Jul 11, 2023**
    - The `apm-data` plugin was introduced in ES v8.12.0 ([#97546](https://github.com/elastic/elasticsearch/pull/97546))
- **Nov 16, 2023**
    - For APM Server, the requirement to install the APM Integrations package was removed in v8.15.0 ([#12066](https://github.com/elastic/apm-server/pull/12066)).
- **May 21, 2024**
    - The APM plugin in ES was only enabled as the default in v8.15.0 ([#108860](https://github.com/elastic/elasticsearch/pull/108860)).
- **May 22, 2024**
    - PR [#108885](https://github.com/elastic/elasticsearch/pull/108885) ensures that templates installed via `apm-data` ES plugin should take precedence over the ones installed by the APM Integrations package.
- **May 26, 2024**
    - In [#9949](https://github.com/elastic/integrations/pull/9949) all datastreams was removed from APM Integrations.

## 8.x - (Fixes & Improvements)

The switch to the ES apm plugin caused several issues for our customers.

- **Sep 11, 2024**
    - Lazy rollover on a data stream is not triggered when writing a document that is rerouted to another data stream, fixed in ES [#112781](https://github.com/elastic/elasticsearch/issues/112781).
- **Sep 12, 2024**
    - Any old datastreams created before the switch would be `Unmanaged` because the datastream will never be updated with the DSL lifecycle.
    - New indices created for clusters which migrate to 8.15.0 don't have any lifecycle attached as existing datastream needs to be updated explicitly, see [Docs](https://www.elastic.co/guide/en/elasticsearch/reference/current/tutorial-manage-existing-data-stream.html).
    - PR [#112759](https://github.com/elastic/elasticsearch/pull/112759) fixes the fallback to legacy ILM policies when a datastream is updated.
- **Oct 25, 2024** 
    - With the new index templates, if you were not using any custom ILM Policy, APM data will obey to the new Data stream Lifecycle instead of ILM.
    - The default ILM Policies of APM are removed if not in use. If you defined a custom ILM policy via a `@custom` component template, the ILM policy will be preserved and preferred to DSL.
    - In [#115687](https://github.com/elastic/elasticsearch/pull/115687), we moved to adding default ILM policies and switch to ILM for apm-data plugin, instead of just having a fallback as outlined in [#112759](https://github.com/elastic/elasticsearch/pull/112759).
- **Nov 5, 2024**
    - PR [#116219](https://github.com/elastic/elasticsearch/pull/116219) will trigger a lazy rollover of existing data streams regardless of whether the index template is being created or updated.
    - This ensures that the apm-data plugin will roll over data streams that were previously using the Fleet integration package.


