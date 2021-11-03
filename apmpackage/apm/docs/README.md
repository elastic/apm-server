# APM Integration

The APM integration installs Elasticsearch templates and ingest node pipelines for APM data.

### Quick start

Ready to jump in? Read the [APM quick start](https://ela.st/quick-start-apm).

### How to use this integration

Add the APM integration to an Elastic Agent policy to create an `apm` input.
Any Elastic Agents set up with this policy will run an APM Server binary locally.
Don't forget to configure the APM Server `host` if it needs to be accessed from outside, like when running in Docker.
Then, configure your APM agents to communicate with APM Server.

If you have Real User Monitoring (RUM) enabled, you must run Elastic Agent centrally.
Otherwise, you can run it on edge machines by downloading and installing Elastic Agent
on the same machines that your instrumented services run.

#### Data Streams

When using the APM integration, apm events are indexed into data streams. Data stream names contain the event type,
service name, and a user-configurable namespace.

There is no specific recommendation for what to use as a namespace; it is intentionally flexible.
You might use the environment, like `production`, `testing`, or `development`,
or you could namespace data by business unit. It is your choice.

See [APM data streams](https://ela.st/apm-data-streams) for more information.

## Compatibility and limitations

The APM integration requires Kibana v7.12 and Elasticsearch with at least the basic license.
This version is experimental and has some limitations, listed bellow:

- Sourcemaps need to be uploaded to Elasticsearch directly.
- You need to create specific API keys for sourcemaps and central configuration.
- You can't use an Elastic Agent enrolled before 7.12.
- Not all settings are supported.
- The `apm` templates, pipelines, and ILM settings that ship with this integration cannot be configured or changed with Fleet;
changes must be made with Elasticsearch APIs or Kibana's Stack Management.

See [APM integration limitations](https://ela.st/apm-integration-limitations) for more information.

IMPORTANT: If you run APM Server with Elastic Agent manually in standalone mode, you must install the APM integration before ingestion starts.

## Traces

Traces are comprised of [spans and transactions](https://www.elastic.co/guide/en/apm/get-started/current/apm-data-model.html).

Traces are written to `traces-apm-*` data streams.

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| agent.ephemeral_id | Ephemeral identifier of this agent (if one exists). This id normally changes across restarts, but `agent.id` does not. | keyword |
| agent.name | Custom name of the agent. This is a name that can be given to an agent. This can be helpful if for example two Filebeat instances are running on the same host but a human readable separation is needed on which Filebeat instance data is coming from. If no name is given, the name is often left empty. | keyword |
| agent.version | Version of the agent. | keyword |
| child.id | The ID(s) of the child event(s). | keyword |
| client.domain | Client domain. | keyword |
| client.geo.city_name | City name. | keyword |
| client.geo.continent_name | Name of the continent. | keyword |
| client.geo.country_iso_code | Country ISO code. | keyword |
| client.geo.country_name | Country name. | keyword |
| client.geo.location | Longitude and latitude. | geo_point |
| client.geo.region_iso_code | Region ISO code. | keyword |
| client.geo.region_name | Region name. | keyword |
| client.ip | IP address of the client (IPv4 or IPv6). | ip |
| client.port | Port of the client. | long |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |
| cloud.account.name | The cloud account name or alias used to identify different entities in a multi-tenant environment. Examples: AWS account name, Google Cloud ORG display name. | keyword |
| cloud.availability_zone | Availability zone in which this host, resource, or service is located. | keyword |
| cloud.instance.id | Instance ID of the host machine. | keyword |
| cloud.instance.name | Instance name of the host machine. | keyword |
| cloud.machine.type | Machine type of the host machine. | keyword |
| cloud.origin.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. | keyword |
| cloud.origin.provider | Name of the cloud provider. | keyword |
| cloud.origin.region | Region in which this host, resource, or service is located. | keyword |
| cloud.origin.service.name | The cloud service name is intended to distinguish services running on different platforms within a provider. | keyword |
| cloud.project.id | The cloud project identifier. Examples: Google Cloud Project id, Azure Project id. | keyword |
| cloud.project.name | The cloud project name. Examples: Google Cloud Project name, Azure Project name. | keyword |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |
| cloud.region | Region in which this host, resource, or service is located. | keyword |
| cloud.service.name | The cloud service name is intended to distinguish services running on different platforms within a provider, eg AWS EC2 vs Lambda, GCP GCE vs App Engine, Azure VM vs App Server. Examples: app engine, app service, cloud run, fargate, lambda. | keyword |
| container.id | Unique container id. | keyword |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| destination.address | Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is. | keyword |
| destination.ip | IP address of the destination (IPv4 or IPv6). | ip |
| destination.port | Port of the destination. | long |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |
| event.outcome | This is one of four ECS Categorization Fields, and indicates the lowest level in the ECS category hierarchy. `event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event. Note that when a single transaction is described in multiple events, each event may populate different values of `event.outcome`, according to their perspective. Also note that in the case of a compound event (a single event that contains multiple logical events), this field should be populated with the value that best captures the overall success or failure from the perspective of the event producer. Further note that not all events will have an associated outcome. For example, this field is generally not populated for metric events, events with `event.type:info`, or any events for which an outcome does not make logical sense. | keyword |
| faas.coldstart | Boolean indicating whether the function invocation was a coldstart or not. | boolean |
| faas.execution | Request ID of the function invocation. | keyword |
| faas.trigger.request_id | The ID of the origin trigger request. | keyword |
| faas.trigger.type | The trigger type. | keyword |
| host.architecture | Operating system architecture. | keyword |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |
| host.ip | Host ip addresses. | ip |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| http.request.headers | The canonical headers of the monitored HTTP request. | object |
| http.request.method | HTTP request method. Prior to ECS 1.6.0 the following guidance was provided: "The field value must be normalized to lowercase for querying." As of ECS 1.6.0, the guidance is deprecated because the original case of the method may be useful in anomaly detection.  Original case will be mandated in ECS 2.0.0 | keyword |
| http.request.referrer | Referrer for this HTTP request. | keyword |
| http.response.finished | Used by the Node agent to indicate when in the response life cycle an error has occurred. | boolean |
| http.response.headers | The canonical headers of the monitored HTTP response. | object |
| http.response.status_code | HTTP response status code. | long |
| http.version | HTTP version. | keyword |
| kubernetes.namespace | Kubernetes namespace | keyword |
| kubernetes.node.name | Kubernetes node name | keyword |
| kubernetes.pod.name | Kubernetes pod name | keyword |
| kubernetes.pod.uid | Kubernetes Pod UID | keyword |
| labels | A flat mapping of user-defined labels with string, boolean or number values. | object |
| network.carrier.icc | ISO country code, eg. US | keyword |
| network.carrier.mcc | Mobile country code | keyword |
| network.carrier.mnc | Mobile network code | keyword |
| network.carrier.name | Carrier name, eg. Vodafone, T-Mobile, etc. | keyword |
| network.connection.subtype | Detailed network connection sub-type, e.g. "LTE", "CDMA" | keyword |
| network.connection.type | Network connection type, eg. "wifi", "cell" | keyword |
| observer.ephemeral_id | Ephemeral identifier of the APM Server. | keyword |
| observer.hostname | Hostname of the observer. | keyword |
| observer.id | Unique identifier of the APM Server. | keyword |
| observer.name | Custom name of the observer. This is a name that can be given to an observer. This can be helpful for example if multiple firewalls of the same model are used in an organization. If no custom name is needed, the field can be left empty. | keyword |
| observer.type | The type of the observer the data is coming from. There is no predefined list of observer types. Some examples are `forwarder`, `firewall`, `ids`, `ips`, `proxy`, `poller`, `sensor`, `APM server`. | keyword |
| observer.version | Observer version. | keyword |
| observer.version_major | Major version number of the observer | byte |
| parent.id | The ID of the parent event. | keyword |
| process.args | Array of process arguments, starting with the absolute path to the executable. May be filtered to protect sensitive information. | keyword |
| process.pid | Process id. | long |
| process.ppid | Parent process' pid. | long |
| process.title | Process title. The proctitle, some times the same as process name. Can also be different: for example a browser setting its title to the web page currently opened. | keyword |
| processor.event | Processor event. | keyword |
| processor.name | Processor name. | constant_keyword |
| service.environment | Identifies the environment where the service is running. If the same service runs in different environments (production, staging, QA, development, etc.), the environment can identify other instances of the same service. Can also group services and applications from the same environment. | keyword |
| service.framework.name | Name of the framework used. | keyword |
| service.framework.version | Version of the framework used. | keyword |
| service.language.name | Name of the programming language used. | keyword |
| service.language.version | Version of the programming language used. | keyword |
| service.name | Name of the service data is collected from. The name of the service is normally user given. This allows for distributed services that run on multiple hosts to correlate the related instances based on the name. In the case of Elasticsearch the `service.name` could contain the cluster name. For Beats the `service.name` is by default a copy of the `service.type` field if no name is specified. | keyword |
| service.node.name | Name of a service node. This allows for two nodes of the same service running on the same host to be differentiated. Therefore, `service.node.name` should typically be unique across nodes of a given service. In the case of Elasticsearch, the `service.node.name` could contain the unique node name within the Elasticsearch cluster. In cases where the service doesn't have the concept of a node name, the host name or container name can be used to distinguish running instances that make up this service. If those do not provide uniqueness (e.g. multiple instances of the service running on the same host) - the node name can be manually set. | keyword |
| service.origin.id | Immutable id of the service emitting this event. | keyword |
| service.origin.name | Immutable name of the service emitting this event. | keyword |
| service.origin.version | The version of the service the data was collected from. | keyword |
| service.runtime.name | Name of the runtime used. | keyword |
| service.runtime.version | Version of the runtime used. | keyword |
| service.version | Version of the service the data was collected from. This allows to look at a data set only for a specific version of a service. | keyword |
| session.id | The ID of the session to which the event belongs. | keyword |
| session.sequence | The sequence number of the event within the session to which the event belongs. | long |
| source.domain | Source domain. | keyword |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |
| source.port | Port of the source. | long |
| span.action | The specific kind of event within the sub-type represented by the span (e.g. query, connect) | keyword |
| span.composite.compression_strategy | The compression strategy that was used. | keyword |
| span.composite.count | Number of compressed spans the composite span represents. | long |
| span.composite.sum.us | Sum of the durations of the compressed spans, in microseconds. | long |
| span.db.link | Database link. | keyword |
| span.db.rows_affected | Number of rows affected by the database statement. | long |
| span.destination.service.name | Identifier for the destination service (e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq') DEPRECATED: this field will be removed in a future release | keyword |
| span.destination.service.resource | Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name') | keyword |
| span.destination.service.type | Type of the destination service (e.g. 'db', 'elasticsearch'). Should typically be the same as span.type. DEPRECATED: this field will be removed in a future release | keyword |
| span.duration.us | Duration of the span, in microseconds. | long |
| span.id | Unique identifier of the span within the scope of its trace. A span represents an operation within a transaction, such as a request to another service, or a database query. | keyword |
| span.kind | "The kind of span: CLIENT, SERVER, PRODUCER, CONSUMER, or INTERNAL." | keyword |
| span.message.age.ms | Age of a message in milliseconds. | long |
| span.message.queue.name | Name of the message queue or topic where the message is published or received. | keyword |
| span.name | Generic designation of a span in the scope of a transaction. | keyword |
| span.start.us | Offset relative to the transaction's timestamp identifying the start of the span, in microseconds. | long |
| span.subtype | A further sub-division of the type (e.g. postgresql, elasticsearch) | keyword |
| span.sync | Indicates whether the span was executed synchronously or asynchronously. | boolean |
| span.type | Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', 'cache', etc). | keyword |
| timestamp.us | Timestamp of the event in microseconds since Unix epoch. | long |
| trace.id | Unique identifier of the trace. A trace groups multiple events like transactions that belong together. For example, a user request handled by multiple inter-connected services. | keyword |
| transaction.duration.us | Total duration of this transaction, in microseconds. | long |
| transaction.experience.cls | The Cumulative Layout Shift metric | scaled_float |
| transaction.experience.fid | The First Input Delay metric | scaled_float |
| transaction.experience.longtask.count | The total number of of longtasks | long |
| transaction.experience.longtask.max | The max longtask duration | scaled_float |
| transaction.experience.longtask.sum | The sum of longtask durations | scaled_float |
| transaction.experience.tbt | The Total Blocking Time metric | scaled_float |
| transaction.id | Unique identifier of the transaction within the scope of its trace. A transaction is the highest level of work measured within a service, such as a request to a server. | keyword |
| transaction.marks | A user-defined mapping of groups of marks in milliseconds. | object |
| transaction.message.age.ms | Age of a message in milliseconds. | long |
| transaction.message.queue.name | Name of the message queue or topic where the message is published or received. | keyword |
| transaction.name | Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id'). | keyword |
| transaction.result | The result of the transaction. HTTP status code for HTTP-related transactions. | keyword |
| transaction.sampled | Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context. | boolean |
| transaction.span_count.dropped | The total amount of dropped spans for this transaction. | long |
| transaction.type | Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc) | keyword |
| url.domain | Domain of the url, such as "www.elastic.co". In some cases a URL may refer to an IP and/or port directly, without a domain name. In this case, the IP address would go to the `domain` field. If the URL contains a literal IPv6 address enclosed by `[` and `]` (IETF RFC 2732), the `[` and `]` characters should also be captured in the `domain` field. | keyword |
| url.fragment | Portion of the url after the `#`, such as "top". The `#` is not part of the fragment. | keyword |
| url.full | If full URLs are important to your use case, they should be stored in `url.full`, whether this field is reconstructed or present in the event source. | wildcard |
| url.path | Path of the request, such as "/search". | wildcard |
| url.port | Port of the request, such as 443. | long |
| url.query | The query field describes the query string of the request, such as "q=elasticsearch". The `?` is excluded from the query string. If a URL contains no `?`, there is no query field. If there is a `?` but no query, the query field exists with an empty string. The `exists` query can be used to differentiate between the two cases. | keyword |
| url.scheme | Scheme of the request, such as "https". Note: The `:` is not part of the scheme. | keyword |
| user.domain | Name of the directory the user is a member of. For example, an LDAP or Active Directory domain name. | keyword |
| user.email | User email address. | keyword |
| user.id | Unique identifier of the user. | keyword |
| user.name | Short name or login of the user. | keyword |
| user_agent.device.name | Name of the device. | keyword |
| user_agent.name | Name of the user agent. | keyword |
| user_agent.original | Unparsed user_agent string. | keyword |
| user_agent.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |
| user_agent.os.full | Operating system name, including the version or code name. | keyword |
| user_agent.os.kernel | Operating system kernel version as a raw string. | keyword |
| user_agent.os.name | Operating system name, without the version. | keyword |
| user_agent.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| user_agent.os.version | Operating system version as a raw string. | keyword |
| user_agent.version | Version of the user agent. | keyword |


## Application Metrics

Application metrics are comprised of custom, application-specific metrics, basic system metrics such as CPU and memory usage,
and runtime metrics such as JVM garbage collection statistics.

Application metrics are written to service-specific `metrics-apm.app.*-*` data streams.

**Exported fields**

| Field | Description | Type | Unit | Metric Type |
|---|---|---|---|---|
| @timestamp | Event timestamp. | date |  |  |
| agent.ephemeral_id | Ephemeral identifier of this agent (if one exists). This id normally changes across restarts, but `agent.id` does not. | keyword |  |  |
| agent.name | Custom name of the agent. This is a name that can be given to an agent. This can be helpful if for example two Filebeat instances are running on the same host but a human readable separation is needed on which Filebeat instance data is coming from. If no name is given, the name is often left empty. | keyword |  |  |
| agent.version | Version of the agent. | keyword |  |  |
| client.domain | Client domain. | keyword |  |  |
| client.geo.city_name | City name. | keyword |  |  |
| client.geo.continent_name | Name of the continent. | keyword |  |  |
| client.geo.country_iso_code | Country ISO code. | keyword |  |  |
| client.geo.country_name | Country name. | keyword |  |  |
| client.geo.location | Longitude and latitude. | geo_point |  |  |
| client.geo.region_iso_code | Region ISO code. | keyword |  |  |
| client.geo.region_name | Region name. | keyword |  |  |
| client.ip | IP address of the client (IPv4 or IPv6). | ip |  |  |
| client.port | Port of the client. | long |  |  |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |  |  |
| cloud.account.name | The cloud account name or alias used to identify different entities in a multi-tenant environment. Examples: AWS account name, Google Cloud ORG display name. | keyword |  |  |
| cloud.availability_zone | Availability zone in which this host, resource, or service is located. | keyword |  |  |
| cloud.instance.id | Instance ID of the host machine. | keyword |  |  |
| cloud.instance.name | Instance name of the host machine. | keyword |  |  |
| cloud.machine.type | Machine type of the host machine. | keyword |  |  |
| cloud.project.id | The cloud project identifier. Examples: Google Cloud Project id, Azure Project id. | keyword |  |  |
| cloud.project.name | The cloud project name. Examples: Google Cloud Project name, Azure Project name. | keyword |  |  |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |  |  |
| cloud.region | Region in which this host, resource, or service is located. | keyword |  |  |
| cloud.service.name | The cloud service name is intended to distinguish services running on different platforms within a provider, eg AWS EC2 vs Lambda, GCP GCE vs App Engine, Azure VM vs App Server. Examples: app engine, app service, cloud run, fargate, lambda. | keyword |  |  |
| container.id | Unique container id. | keyword |  |  |
| data_stream.dataset | Data stream dataset. | constant_keyword |  |  |
| data_stream.namespace | Data stream namespace. | constant_keyword |  |  |
| data_stream.type | Data stream type. | constant_keyword |  |  |
| destination.address | Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is. | keyword |  |  |
| destination.ip | IP address of the destination (IPv4 or IPv6). | ip |  |  |
| destination.port | Port of the destination. | long |  |  |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |  |  |
| event.outcome | This is one of four ECS Categorization Fields, and indicates the lowest level in the ECS category hierarchy. `event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event. Note that when a single transaction is described in multiple events, each event may populate different values of `event.outcome`, according to their perspective. Also note that in the case of a compound event (a single event that contains multiple logical events), this field should be populated with the value that best captures the overall success or failure from the perspective of the event producer. Further note that not all events will have an associated outcome. For example, this field is generally not populated for metric events, events with `event.type:info`, or any events for which an outcome does not make logical sense. | keyword |  |  |
| host.architecture | Operating system architecture. | keyword |  |  |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |  |  |
| host.ip | Host ip addresses. | ip |  |  |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |  |  |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |  |  |
| kubernetes.namespace | Kubernetes namespace | keyword |  |  |
| kubernetes.node.name | Kubernetes node name | keyword |  |  |
| kubernetes.pod.name | Kubernetes pod name | keyword |  |  |
| kubernetes.pod.uid | Kubernetes Pod UID | keyword |  |  |
| labels | A flat mapping of user-defined labels with string, boolean or number values. | object |  |  |
| metricset.name | Name of the set of metrics. | keyword |  |  |
| network.connection.type | Network connection type, eg. "wifi", "cell" | keyword |  |  |
| observer.ephemeral_id | Ephemeral identifier of the APM Server. | keyword |  |  |
| observer.hostname | Hostname of the observer. | keyword |  |  |
| observer.id | Unique identifier of the APM Server. | keyword |  |  |
| observer.name | Custom name of the observer. This is a name that can be given to an observer. This can be helpful for example if multiple firewalls of the same model are used in an organization. If no custom name is needed, the field can be left empty. | keyword |  |  |
| observer.type | The type of the observer the data is coming from. There is no predefined list of observer types. Some examples are `forwarder`, `firewall`, `ids`, `ips`, `proxy`, `poller`, `sensor`, `APM server`. | keyword |  |  |
| observer.version | Observer version. | keyword |  |  |
| observer.version_major | Major version number of the observer | byte |  |  |
| process.args | Array of process arguments, starting with the absolute path to the executable. May be filtered to protect sensitive information. | keyword |  |  |
| process.pid | Process id. | long |  |  |
| process.ppid | Parent process' pid. | long |  |  |
| process.title | Process title. The proctitle, some times the same as process name. Can also be different: for example a browser setting its title to the web page currently opened. | keyword |  |  |
| processor.event | Processor event. | constant_keyword |  |  |
| processor.name | Processor name. | constant_keyword |  |  |
| service.environment | Identifies the environment where the service is running. If the same service runs in different environments (production, staging, QA, development, etc.), the environment can identify other instances of the same service. Can also group services and applications from the same environment. | keyword |  |  |
| service.framework.name | Name of the framework used. | keyword |  |  |
| service.framework.version | Version of the framework used. | keyword |  |  |
| service.language.name | Name of the programming language used. | keyword |  |  |
| service.language.version | Version of the programming language used. | keyword |  |  |
| service.name | Name of the service data is collected from. The name of the service is normally user given. This allows for distributed services that run on multiple hosts to correlate the related instances based on the name. In the case of Elasticsearch the `service.name` could contain the cluster name. For Beats the `service.name` is by default a copy of the `service.type` field if no name is specified. | keyword |  |  |
| service.node.name | Name of a service node. This allows for two nodes of the same service running on the same host to be differentiated. Therefore, `service.node.name` should typically be unique across nodes of a given service. In the case of Elasticsearch, the `service.node.name` could contain the unique node name within the Elasticsearch cluster. In cases where the service doesn't have the concept of a node name, the host name or container name can be used to distinguish running instances that make up this service. If those do not provide uniqueness (e.g. multiple instances of the service running on the same host) - the node name can be manually set. | keyword |  |  |
| service.runtime.name | Name of the runtime used. | keyword |  |  |
| service.runtime.version | Version of the runtime used. | keyword |  |  |
| service.version | Version of the service the data was collected from. This allows to look at a data set only for a specific version of a service. | keyword |  |  |
| source.domain | Source domain. | keyword |  |  |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |  |  |
| source.port | Port of the source. | long |  |  |
| system.cpu.total.norm.pct | The percentage of CPU time spent by the process since the last event. This value is normalized by the number of CPU cores and it ranges from 0 to 100%. | scaled_float | percent | gauge |
| system.memory.actual.free | Actual free memory in bytes. It is calculated based on the OS. On Linux it consists of the free memory plus caches and buffers. On OSX it is a sum of free memory and the inactive memory. On Windows, it is equal to `system.memory.free`. | long | byte | gauge |
| system.memory.total | Total memory. | long | byte | gauge |
| system.process.cgroup.cpu.cfs.period.us | CFS period in microseconds. | long | micros | gauge |
| system.process.cgroup.cpu.cfs.quota.us | CFS quota in microseconds. | long | micros | gauge |
| system.process.cgroup.cpu.id | ID for the current cgroup CPU. | keyword |  |  |
| system.process.cgroup.cpu.stats.periods | Number of periods seen by the CPU. | long |  | counter |
| system.process.cgroup.cpu.stats.throttled.ns | Nanoseconds spent throttled seen by the CPU. | long | nanos | counter |
| system.process.cgroup.cpu.stats.throttled.periods | Number of throttled periods seen by the CPU. | long |  | counter |
| system.process.cgroup.cpuacct.id | ID for the current cgroup CPU. | keyword |  |  |
| system.process.cgroup.cpuacct.total.ns | Total CPU time for the current cgroup CPU in nanoseconds. | long | nanos | counter |
| system.process.cgroup.memory.mem.limit.bytes | Memory limit for the current cgroup slice. | long | byte | gauge |
| system.process.cgroup.memory.mem.usage.bytes | Memory usage by the current cgroup slice. | long | byte | gauge |
| system.process.cpu.total.norm.pct | The percentage of CPU time spent by the process since the last event. This value is normalized by the number of CPU cores and it ranges from 0 to 100%. | scaled_float | percent | gauge |
| system.process.memory.rss.bytes | The Resident Set Size. The amount of memory the process occupied in main memory (RAM). | long | byte | gauge |
| system.process.memory.size | The total virtual memory the process has. | long | byte | gauge |
| user.domain | Name of the directory the user is a member of. For example, an LDAP or Active Directory domain name. | keyword |  |  |
| user.email | User email address. | keyword |  |  |
| user.id | Unique identifier of the user. | keyword |  |  |
| user.name | Short name or login of the user. | keyword |  |  |
| user_agent.device.name | Name of the device. | keyword |  |  |
| user_agent.name | Name of the user agent. | keyword |  |  |
| user_agent.original | Unparsed user_agent string. | keyword |  |  |
| user_agent.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |  |  |
| user_agent.os.full | Operating system name, including the version or code name. | keyword |  |  |
| user_agent.os.kernel | Operating system kernel version as a raw string. | keyword |  |  |
| user_agent.os.name | Operating system name, without the version. | keyword |  |  |
| user_agent.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |  |  |
| user_agent.os.version | Operating system version as a raw string. | keyword |  |  |
| user_agent.version | Version of the user agent. | keyword |  |  |


## Internal Metrics

Internal metrics comprises metrics produced by Elastic APM agents and Elastic APM server for powering various Kibana charts
in the APM app, such as "Time spent by span type".

Internal metrics are written to `metrics-apm.internal-*` data streams.

**Exported fields**

| Field | Description | Type | Unit |
|---|---|---|---|
| @timestamp | Event timestamp. | date |  |
| agent.ephemeral_id | Ephemeral identifier of this agent (if one exists). This id normally changes across restarts, but `agent.id` does not. | keyword |  |
| agent.name | Custom name of the agent. This is a name that can be given to an agent. This can be helpful if for example two Filebeat instances are running on the same host but a human readable separation is needed on which Filebeat instance data is coming from. If no name is given, the name is often left empty. | keyword |  |
| agent.version | Version of the agent. | keyword |  |
| client.domain | Client domain. | keyword |  |
| client.geo.city_name | City name. | keyword |  |
| client.geo.continent_name | Name of the continent. | keyword |  |
| client.geo.country_iso_code | Country ISO code. | keyword |  |
| client.geo.country_name | Country name. | keyword |  |
| client.geo.location | Longitude and latitude. | geo_point |  |
| client.geo.region_iso_code | Region ISO code. | keyword |  |
| client.geo.region_name | Region name. | keyword |  |
| client.ip | IP address of the client (IPv4 or IPv6). | ip |  |
| client.port | Port of the client. | long |  |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |  |
| cloud.account.name | The cloud account name or alias used to identify different entities in a multi-tenant environment. Examples: AWS account name, Google Cloud ORG display name. | keyword |  |
| cloud.availability_zone | Availability zone in which this host, resource, or service is located. | keyword |  |
| cloud.instance.id | Instance ID of the host machine. | keyword |  |
| cloud.instance.name | Instance name of the host machine. | keyword |  |
| cloud.machine.type | Machine type of the host machine. | keyword |  |
| cloud.project.id | The cloud project identifier. Examples: Google Cloud Project id, Azure Project id. | keyword |  |
| cloud.project.name | The cloud project name. Examples: Google Cloud Project name, Azure Project name. | keyword |  |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |  |
| cloud.region | Region in which this host, resource, or service is located. | keyword |  |
| cloud.service.name | The cloud service name is intended to distinguish services running on different platforms within a provider, eg AWS EC2 vs Lambda, GCP GCE vs App Engine, Azure VM vs App Server. Examples: app engine, app service, cloud run, fargate, lambda. | keyword |  |
| container.id | Unique container id. | keyword |  |
| data_stream.dataset | Data stream dataset. | constant_keyword |  |
| data_stream.namespace | Data stream namespace. | constant_keyword |  |
| data_stream.type | Data stream type. | constant_keyword |  |
| destination.address | Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is. | keyword |  |
| destination.ip | IP address of the destination (IPv4 or IPv6). | ip |  |
| destination.port | Port of the destination. | long |  |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |  |
| event.outcome | This is one of four ECS Categorization Fields, and indicates the lowest level in the ECS category hierarchy. `event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event. Note that when a single transaction is described in multiple events, each event may populate different values of `event.outcome`, according to their perspective. Also note that in the case of a compound event (a single event that contains multiple logical events), this field should be populated with the value that best captures the overall success or failure from the perspective of the event producer. Further note that not all events will have an associated outcome. For example, this field is generally not populated for metric events, events with `event.type:info`, or any events for which an outcome does not make logical sense. | keyword |  |
| host.architecture | Operating system architecture. | keyword |  |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |  |
| host.ip | Host ip addresses. | ip |  |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |  |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |  |
| kubernetes.namespace | Kubernetes namespace | keyword |  |
| kubernetes.node.name | Kubernetes node name | keyword |  |
| kubernetes.pod.name | Kubernetes pod name | keyword |  |
| kubernetes.pod.uid | Kubernetes Pod UID | keyword |  |
| labels | A flat mapping of user-defined labels with string, boolean or number values. | object |  |
| metricset.name | Name of the set of metrics. | keyword |  |
| network.connection.type | Network connection type, eg. "wifi", "cell" | keyword |  |
| observer.ephemeral_id | Ephemeral identifier of the APM Server. | keyword |  |
| observer.hostname | Hostname of the observer. | keyword |  |
| observer.id | Unique identifier of the APM Server. | keyword |  |
| observer.name | Custom name of the observer. This is a name that can be given to an observer. This can be helpful for example if multiple firewalls of the same model are used in an organization. If no custom name is needed, the field can be left empty. | keyword |  |
| observer.type | The type of the observer the data is coming from. There is no predefined list of observer types. Some examples are `forwarder`, `firewall`, `ids`, `ips`, `proxy`, `poller`, `sensor`, `APM server`. | keyword |  |
| observer.version | Observer version. | keyword |  |
| observer.version_major | Major version number of the observer | byte |  |
| process.args | Array of process arguments, starting with the absolute path to the executable. May be filtered to protect sensitive information. | keyword |  |
| process.pid | Process id. | long |  |
| process.ppid | Parent process' pid. | long |  |
| process.title | Process title. The proctitle, some times the same as process name. Can also be different: for example a browser setting its title to the web page currently opened. | keyword |  |
| processor.event | Processor event. | constant_keyword |  |
| processor.name | Processor name. | constant_keyword |  |
| service.environment | Identifies the environment where the service is running. If the same service runs in different environments (production, staging, QA, development, etc.), the environment can identify other instances of the same service. Can also group services and applications from the same environment. | keyword |  |
| service.framework.name | Name of the framework used. | keyword |  |
| service.framework.version | Version of the framework used. | keyword |  |
| service.language.name | Name of the programming language used. | keyword |  |
| service.language.version | Version of the programming language used. | keyword |  |
| service.name | Name of the service data is collected from. The name of the service is normally user given. This allows for distributed services that run on multiple hosts to correlate the related instances based on the name. In the case of Elasticsearch the `service.name` could contain the cluster name. For Beats the `service.name` is by default a copy of the `service.type` field if no name is specified. | keyword |  |
| service.node.name | Name of a service node. This allows for two nodes of the same service running on the same host to be differentiated. Therefore, `service.node.name` should typically be unique across nodes of a given service. In the case of Elasticsearch, the `service.node.name` could contain the unique node name within the Elasticsearch cluster. In cases where the service doesn't have the concept of a node name, the host name or container name can be used to distinguish running instances that make up this service. If those do not provide uniqueness (e.g. multiple instances of the service running on the same host) - the node name can be manually set. | keyword |  |
| service.runtime.name | Name of the runtime used. | keyword |  |
| service.runtime.version | Version of the runtime used. | keyword |  |
| service.version | Version of the service the data was collected from. This allows to look at a data set only for a specific version of a service. | keyword |  |
| source.domain | Source domain. | keyword |  |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |  |
| source.port | Port of the source. | long |  |
| span.destination.service.resource | Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name') | keyword |  |
| span.destination.service.response_time.count | Number of aggregated outgoing requests. | long |  |
| span.destination.service.response_time.sum.us | Aggregated duration of outgoing requests, in microseconds. | long | micros |
| span.self_time.count | Number of aggregated spans. | long |  |
| span.self_time.sum.us | Aggregated span duration, excluding the time periods where a direct child was running, in microseconds. | long | micros |
| span.subtype | A further sub-division of the type (e.g. postgresql, elasticsearch) | keyword |  |
| span.type | Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', 'cache', etc). | keyword |  |
| timeseries.instance | Time series instance ID | keyword |  |
| transaction.duration.histogram | Pre-aggregated histogram of transaction durations. | histogram |  |
| transaction.name | Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id'). | keyword |  |
| transaction.result | The result of the transaction. HTTP status code for HTTP-related transactions. | keyword |  |
| transaction.root | Identifies metrics for root transactions. This can be used for calculating metrics for traces. | boolean |  |
| transaction.sampled | Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context. | boolean |  |
| transaction.self_time.count | Number of aggregated transactions. | long |  |
| transaction.self_time.sum.us | Aggregated transaction duration, excluding the time periods where a direct child was running, in microseconds. | long | micros |
| transaction.type | Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc) | keyword |  |
| user.domain | Name of the directory the user is a member of. For example, an LDAP or Active Directory domain name. | keyword |  |
| user.email | User email address. | keyword |  |
| user.id | Unique identifier of the user. | keyword |  |
| user.name | Short name or login of the user. | keyword |  |
| user_agent.device.name | Name of the device. | keyword |  |
| user_agent.name | Name of the user agent. | keyword |  |
| user_agent.original | Unparsed user_agent string. | keyword |  |
| user_agent.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |  |
| user_agent.os.full | Operating system name, including the version or code name. | keyword |  |
| user_agent.os.kernel | Operating system kernel version as a raw string. | keyword |  |
| user_agent.os.name | Operating system name, without the version. | keyword |  |
| user_agent.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |  |
| user_agent.os.version | Operating system version as a raw string. | keyword |  |
| user_agent.version | Version of the user agent. | keyword |  |


## Application errors

Application errors comprises error/exception events occurring in an application.

Application errors are written to `logs-apm.error.*` data stream.

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| agent.ephemeral_id | Ephemeral identifier of this agent (if one exists). This id normally changes across restarts, but `agent.id` does not. | keyword |
| agent.name | Custom name of the agent. This is a name that can be given to an agent. This can be helpful if for example two Filebeat instances are running on the same host but a human readable separation is needed on which Filebeat instance data is coming from. If no name is given, the name is often left empty. | keyword |
| agent.version | Version of the agent. | keyword |
| client.domain | Client domain. | keyword |
| client.geo.city_name | City name. | keyword |
| client.geo.continent_name | Name of the continent. | keyword |
| client.geo.country_iso_code | Country ISO code. | keyword |
| client.geo.country_name | Country name. | keyword |
| client.geo.location | Longitude and latitude. | geo_point |
| client.geo.region_iso_code | Region ISO code. | keyword |
| client.geo.region_name | Region name. | keyword |
| client.ip | IP address of the client (IPv4 or IPv6). | ip |
| client.port | Port of the client. | long |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |
| cloud.account.name | The cloud account name or alias used to identify different entities in a multi-tenant environment. Examples: AWS account name, Google Cloud ORG display name. | keyword |
| cloud.availability_zone | Availability zone in which this host, resource, or service is located. | keyword |
| cloud.instance.id | Instance ID of the host machine. | keyword |
| cloud.instance.name | Instance name of the host machine. | keyword |
| cloud.machine.type | Machine type of the host machine. | keyword |
| cloud.project.id | The cloud project identifier. Examples: Google Cloud Project id, Azure Project id. | keyword |
| cloud.project.name | The cloud project name. Examples: Google Cloud Project name, Azure Project name. | keyword |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |
| cloud.region | Region in which this host, resource, or service is located. | keyword |
| cloud.service.name | The cloud service name is intended to distinguish services running on different platforms within a provider, eg AWS EC2 vs Lambda, GCP GCE vs App Engine, Azure VM vs App Server. Examples: app engine, app service, cloud run, fargate, lambda. | keyword |
| container.id | Unique container id. | keyword |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| destination.address | Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is. | keyword |
| destination.ip | IP address of the destination (IPv4 or IPv6). | ip |
| destination.port | Port of the destination. | long |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |
| error.culprit | Function call which was the primary perpetrator of this event. | keyword |
| error.exception.code | The error code set when the error happened, e.g. database error code. | keyword |
| error.exception.handled | Indicator whether the error was caught somewhere in the code or not. | boolean |
| error.exception.message | The original error message. | text |
| error.exception.module | The module namespace of the original error. | keyword |
| error.exception.type | The type of the original error, e.g. the Java exception class name. | keyword |
| error.grouping_key | Hash of select properties of the logged error for grouping purposes. | keyword |
| error.grouping_name | Name to associate with an error group. Errors belonging to the same group (same grouping_key) may have differing values for grouping_name. Consumers may choose one arbitrarily. | keyword |
| error.log.level | The severity of the record. | keyword |
| error.log.logger_name | The name of the logger instance used. | keyword |
| error.log.message | The additionally logged error message. | text |
| error.log.param_message | A parametrized message. E.g. 'Could not connect to %s'. The property message is still required, and should be equal to the param_message, but with placeholders replaced. In some situations the param_message is used to group errors together. | keyword |
| event.outcome | This is one of four ECS Categorization Fields, and indicates the lowest level in the ECS category hierarchy. `event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event. Note that when a single transaction is described in multiple events, each event may populate different values of `event.outcome`, according to their perspective. Also note that in the case of a compound event (a single event that contains multiple logical events), this field should be populated with the value that best captures the overall success or failure from the perspective of the event producer. Further note that not all events will have an associated outcome. For example, this field is generally not populated for metric events, events with `event.type:info`, or any events for which an outcome does not make logical sense. | keyword |
| host.architecture | Operating system architecture. | keyword |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |
| host.ip | Host ip addresses. | ip |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| http.request.headers | The canonical headers of the monitored HTTP request. | object |
| http.request.method | HTTP request method. Prior to ECS 1.6.0 the following guidance was provided: "The field value must be normalized to lowercase for querying." As of ECS 1.6.0, the guidance is deprecated because the original case of the method may be useful in anomaly detection.  Original case will be mandated in ECS 2.0.0 | keyword |
| http.request.referrer | Referrer for this HTTP request. | keyword |
| http.response.finished | Used by the Node agent to indicate when in the response life cycle an error has occurred. | boolean |
| http.response.headers | The canonical headers of the monitored HTTP response. | object |
| http.response.status_code | HTTP response status code. | long |
| http.version | HTTP version. | keyword |
| kubernetes.namespace | Kubernetes namespace | keyword |
| kubernetes.node.name | Kubernetes node name | keyword |
| kubernetes.pod.name | Kubernetes pod name | keyword |
| kubernetes.pod.uid | Kubernetes Pod UID | keyword |
| labels | A flat mapping of user-defined labels with string, boolean or number values. | object |
| message | For log events the message field contains the log message, optimized for viewing in a log viewer. For structured logs without an original message field, other fields can be concatenated to form a human-readable summary of the event. If multiple messages exist, they can be combined into one message. | match_only_text |
| network.carrier.icc | ISO country code, eg. US | keyword |
| network.carrier.mcc | Mobile country code | keyword |
| network.carrier.mnc | Mobile network code | keyword |
| network.carrier.name | Carrier name, eg. Vodafone, T-Mobile, etc. | keyword |
| network.connection.subtype | Detailed network connection sub-type, e.g. "LTE", "CDMA" | keyword |
| network.connection.type | Network connection type, eg. "wifi", "cell" | keyword |
| observer.ephemeral_id | Ephemeral identifier of the APM Server. | keyword |
| observer.hostname | Hostname of the observer. | keyword |
| observer.id | Unique identifier of the APM Server. | keyword |
| observer.name | Custom name of the observer. This is a name that can be given to an observer. This can be helpful for example if multiple firewalls of the same model are used in an organization. If no custom name is needed, the field can be left empty. | keyword |
| observer.type | The type of the observer the data is coming from. There is no predefined list of observer types. Some examples are `forwarder`, `firewall`, `ids`, `ips`, `proxy`, `poller`, `sensor`, `APM server`. | keyword |
| observer.version | Observer version. | keyword |
| observer.version_major | Major version number of the observer | byte |
| parent.id | The ID of the parent event. | keyword |
| process.args | Array of process arguments, starting with the absolute path to the executable. May be filtered to protect sensitive information. | keyword |
| process.pid | Process id. | long |
| process.ppid | Parent process' pid. | long |
| process.title | Process title. The proctitle, some times the same as process name. Can also be different: for example a browser setting its title to the web page currently opened. | keyword |
| processor.event | Processor event. | constant_keyword |
| processor.name | Processor name. | constant_keyword |
| service.environment | Identifies the environment where the service is running. If the same service runs in different environments (production, staging, QA, development, etc.), the environment can identify other instances of the same service. Can also group services and applications from the same environment. | keyword |
| service.framework.name | Name of the framework used. | keyword |
| service.framework.version | Version of the framework used. | keyword |
| service.language.name | Name of the programming language used. | keyword |
| service.language.version | Version of the programming language used. | keyword |
| service.name | Name of the service data is collected from. The name of the service is normally user given. This allows for distributed services that run on multiple hosts to correlate the related instances based on the name. In the case of Elasticsearch the `service.name` could contain the cluster name. For Beats the `service.name` is by default a copy of the `service.type` field if no name is specified. | keyword |
| service.node.name | Name of a service node. This allows for two nodes of the same service running on the same host to be differentiated. Therefore, `service.node.name` should typically be unique across nodes of a given service. In the case of Elasticsearch, the `service.node.name` could contain the unique node name within the Elasticsearch cluster. In cases where the service doesn't have the concept of a node name, the host name or container name can be used to distinguish running instances that make up this service. If those do not provide uniqueness (e.g. multiple instances of the service running on the same host) - the node name can be manually set. | keyword |
| service.runtime.name | Name of the runtime used. | keyword |
| service.runtime.version | Version of the runtime used. | keyword |
| service.version | Version of the service the data was collected from. This allows to look at a data set only for a specific version of a service. | keyword |
| source.domain | Source domain. | keyword |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |
| source.port | Port of the source. | long |
| span.id | Unique identifier of the span within the scope of its trace. A span represents an operation within a transaction, such as a request to another service, or a database query. | keyword |
| timestamp.us | Timestamp of the event in microseconds since Unix epoch. | long |
| trace.id | Unique identifier of the trace. A trace groups multiple events like transactions that belong together. For example, a user request handled by multiple inter-connected services. | keyword |
| transaction.id | Unique identifier of the transaction within the scope of its trace. A transaction is the highest level of work measured within a service, such as a request to a server. | keyword |
| transaction.sampled | Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context. | boolean |
| transaction.type | Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc) | keyword |
| url.domain | Domain of the url, such as "www.elastic.co". In some cases a URL may refer to an IP and/or port directly, without a domain name. In this case, the IP address would go to the `domain` field. If the URL contains a literal IPv6 address enclosed by `[` and `]` (IETF RFC 2732), the `[` and `]` characters should also be captured in the `domain` field. | keyword |
| url.fragment | Portion of the url after the `#`, such as "top". The `#` is not part of the fragment. | keyword |
| url.full | If full URLs are important to your use case, they should be stored in `url.full`, whether this field is reconstructed or present in the event source. | wildcard |
| url.path | Path of the request, such as "/search". | wildcard |
| url.port | Port of the request, such as 443. | long |
| url.query | The query field describes the query string of the request, such as "q=elasticsearch". The `?` is excluded from the query string. If a URL contains no `?`, there is no query field. If there is a `?` but no query, the query field exists with an empty string. The `exists` query can be used to differentiate between the two cases. | keyword |
| url.scheme | Scheme of the request, such as "https". Note: The `:` is not part of the scheme. | keyword |
| user.domain | Name of the directory the user is a member of. For example, an LDAP or Active Directory domain name. | keyword |
| user.email | User email address. | keyword |
| user.id | Unique identifier of the user. | keyword |
| user.name | Short name or login of the user. | keyword |
| user_agent.device.name | Name of the device. | keyword |
| user_agent.name | Name of the user agent. | keyword |
| user_agent.original | Unparsed user_agent string. | keyword |
| user_agent.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |
| user_agent.os.full | Operating system name, including the version or code name. | keyword |
| user_agent.os.kernel | Operating system kernel version as a raw string. | keyword |
| user_agent.os.name | Operating system name, without the version. | keyword |
| user_agent.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| user_agent.os.version | Operating system version as a raw string. | keyword |
| user_agent.version | Version of the user agent. | keyword |

