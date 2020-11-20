# APM Integration

Lorem ipsum descriptium

## Compatibility

Dragons

## Configuration parameters

Maybe RUM?

## Traces

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
|processor.name|Processor name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.event|Processor event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|timestamp.us|Timestamp of the event in microseconds since Unix epoch.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|url.scheme|The protocol of the request, e.g. "https:".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.full|The full, possibly agent-assembled URL of the request, e.g https://example.com:443/search?q=elasticsearch#top.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.domain|The hostname of the request, e.g. "example.com".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.port|The port of the request, e.g. 443.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.path|The path of the request, e.g. "/search".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.query|The query string of the request, e.g. "q=elasticsearch".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.fragment|A fragment specifying a location in a web page , e.g. "top".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.version|The http version of the request leading to this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.request.method|The http method of the request leading to this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.request.headers|The canonical headers of the monitored HTTP request.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|http.request.referrer|Referrer for this HTTP request.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.response.status_code|The status code of the HTTP response.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.response.finished|Used by the Node agent to indicate when in the response life cycle an error has occurred.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|http.response.headers|The canonical headers of the monitored HTTP response.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|labels|A flat mapping of user-defined labels with string, boolean or number values.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.name|Immutable name of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.version|Version of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.environment|Service environment.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.node.name|Unique meaningful name of the service node.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.language.name|Name of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.language.version|Version of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.name|Name of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.version|Version of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.name|Name of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.version|Version of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.id|The transaction ID.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|transaction.sampled|Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.type|Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.name|Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id').|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.breakdown.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.type|Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', 'cache', etc).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.subtype|A further sub-division of the type (e.g. postgresql, elasticsearch)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|trace.id|The ID of the trace to which the event belongs to.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|parent.id|The ID of the parent event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|agent.name|Name of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.version|Version of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.ephemeral_id|The Ephemeral ID identifies a running process.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|container.id|Unique container id.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|kubernetes.namespace|Kubernetes namespace|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.node.name|Kubernetes node name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.name|Kubernetes pod name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.uid|Kubernetes Pod UID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|host.architecture|The architecture of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.hostname|The hostname of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.name|Name of the host the event was recorded on. It can contain same information as host.hostname or a name specified by the user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.ip|IP of the host that records the event.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.os.platform|The platform of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.args|Process arguments. May be filtered to protect sensitive information.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.pid|Numeric process ID of the service process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.ppid|Numeric ID of the service's parent process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.title|Service process title.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.listening|Address the server is listening on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.hostname|Hostname of the APM Server.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version|APM Server version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version_major|Major version number of the observer|byte|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.type|The type will be set to `apm-server`.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.name|The username of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.id|Identifier of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.email|Email of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|client.ip|IP address of the client of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|source.ip|IP address of the source of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.address|Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.ip|IP addess of the destination. Can be one of multiple IPv4 or IPv6 addresses.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.port|Port of the destination.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.original|Unparsed version of the user_agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.name|Name of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.version|Version of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.device.name|Name of the device.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.platform|Operating system platform (such centos, ubuntu, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.name|Operating system name, without the version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.full|Operating system name, including the version or code name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.family|OS family (such as redhat, debian, freebsd, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.version|Operating system version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.kernel|Operating system kernel version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|experimental|Additional experimental data sent by the agents.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|cloud.account.id|Cloud account ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.account.name|Cloud account name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.availability_zone|Cloud availability zone name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.id|Cloud instance/machine ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.name|Cloud instance/machine name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.machine.type|Cloud instance/machine type|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.id|Cloud project ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.name|Cloud project name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.provider|Cloud provider name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.region|Cloud region name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|event.outcome|`event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|transaction.duration.us|Total duration of this transaction, in microseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.result|The result of the transaction. HTTP status code for HTTP-related transactions.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.marks|A user-defined mapping of groups of marks in milliseconds.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.marks.*.*||object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.cls|The Cumulative Layout Shift metric|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.fid|The First Input Delay metric|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.tbt|The Total Blocking Time metric|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.longtask.count|The total number of of longtasks|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.longtask.sum|The sum of longtask durations|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.experience.longtask.max|The max longtask duration|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.span_count.dropped|The total amount of dropped spans for this transaction.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.message.queue.name|Name of the message queue or topic where the message is published or received.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.message.age.ms|Age of a message in milliseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.name|Processor name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.event|Processor event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|timestamp.us|Timestamp of the event in microseconds since Unix epoch.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|labels|A flat mapping of user-defined labels with string, boolean or number values.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.name|Immutable name of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.version|Version of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.environment|Service environment.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.node.name|Unique meaningful name of the service node.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.language.name|Name of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.language.version|Version of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.name|Name of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.version|Version of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.name|Name of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.version|Version of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.id|The transaction ID.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|transaction.sampled|Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.type|Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.name|Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id').|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.breakdown.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.type|Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', 'cache', etc).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.subtype|A further sub-division of the type (e.g. postgresql, elasticsearch)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|trace.id|The ID of the trace to which the event belongs to.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|parent.id|The ID of the parent event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|agent.name|Name of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.version|Version of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.ephemeral_id|The Ephemeral ID identifies a running process.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|container.id|Unique container id.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|kubernetes.namespace|Kubernetes namespace|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.node.name|Kubernetes node name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.name|Kubernetes pod name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.uid|Kubernetes Pod UID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|host.architecture|The architecture of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.hostname|The hostname of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.name|Name of the host the event was recorded on. It can contain same information as host.hostname or a name specified by the user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.ip|IP of the host that records the event.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.os.platform|The platform of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.args|Process arguments. May be filtered to protect sensitive information.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.pid|Numeric process ID of the service process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.ppid|Numeric ID of the service's parent process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.title|Service process title.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.listening|Address the server is listening on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.hostname|Hostname of the APM Server.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version|APM Server version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version_major|Major version number of the observer|byte|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.type|The type will be set to `apm-server`.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.name|The username of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.id|Identifier of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.email|Email of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|client.ip|IP address of the client of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|source.ip|IP address of the source of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.address|Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.ip|IP addess of the destination. Can be one of multiple IPv4 or IPv6 addresses.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.port|Port of the destination.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.original|Unparsed version of the user_agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.name|Name of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.version|Version of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.device.name|Name of the device.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.platform|Operating system platform (such centos, ubuntu, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.name|Operating system name, without the version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.full|Operating system name, including the version or code name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.family|OS family (such as redhat, debian, freebsd, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.version|Operating system version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.kernel|Operating system kernel version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|experimental|Additional experimental data sent by the agents.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|cloud.account.id|Cloud account ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.account.name|Cloud account name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.availability_zone|Cloud availability zone name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.id|Cloud instance/machine ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.name|Cloud instance/machine name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.machine.type|Cloud instance/machine type|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.id|Cloud project ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.name|Cloud project name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.provider|Cloud provider name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.region|Cloud region name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|event.outcome|`event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|view spans||keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|child.id|The ID(s)s of the child event(s).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.id|The ID of the span stored as hex encoded string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|span.name|Generic designation of a span in the scope of a transaction.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.action|The specific kind of event within the sub-type represented by the span (e.g. query, connect)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.start.us|Offset relative to the transaction's timestamp identifying the start of the span, in microseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.duration.us|Duration of the span, in microseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.sync|Indicates whether the span was executed synchronously or asynchronously.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.db.link|Database link.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.db.rows_affected|Number of rows affected by the database statement.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.destination.service.type|Type of the destination service (e.g. 'db', 'elasticsearch'). Should typically be the same as span.type.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.destination.service.name|Identifier for the destination service (e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq')|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.destination.service.resource|Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name')|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.message.queue.name|Name of the message queue or topic where the message is published or received.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.message.age.ms|Age of a message in milliseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |


### Example

```json
{
  "@timestamp": "2017-05-30T18:53:42.281Z",
  "agent": {
    "name": "elastic-node",
    "version": "3.14.0"
  },
  "container": {
    "id": "container-id"
  },
  "ecs": {
    "version": "1.6.0"
  },
  "event": {
    "ingested": "2020-08-11T09:55:04.391451Z",
    "outcome": "unknown"
  },
  "host": {
    "architecture": "x64",
    "ip": "127.0.0.1",
    "os": {
      "platform": "darwin"
    }
  },
  "kubernetes": {
    "namespace": "namespace1",
    "pod": {
      "name": "pod-name",
      "uid": "pod-uid"
    }
  },
  "observer": {
    "ephemeral_id": "f78f6762-2157-4322-95aa-aecd2f486c1a",
    "hostname": "ix.lan",
    "id": "80b79979-4a7d-450d-b2ce-75c589f7fffd",
    "type": "apm-server",
    "version": "8.0.0",
    "version_major": 8
  },
  "process": {
    "args": [
      "node",
      "server.js"
    ],
    "pid": 1234,
    "ppid": 6789,
    "title": "node"
  },
  "processor": {
    "event": "transaction",
    "name": "transaction"
  },
  "service": {
    "environment": "staging",
    "framework": {
      "name": "Express",
      "version": "1.2.3"
    },
    "language": {
      "name": "ecmascript",
      "version": "8"
    },
    "name": "1234_service-12a3",
    "node": {
      "name": "container-id"
    },
    "runtime": {
      "name": "node",
      "version": "8.0.0"
    },
    "version": "5.1.3"
  },
  "timestamp": {
    "us": 1496170422281000
  },
  "trace": {
    "id": "85925e55b43f4340aaaaaaaaaaaaaaaa"
  },
  "transaction": {
    "duration": {
      "us": 13980
    },
    "id": "85925e55b43f4340",
    "name": "GET /api/types",
    "result": "failure",
    "sampled": true,
    "span_count": {
      "started": 0
    },
    "type": "request"
  },
  "user": {
    "email": "foo@bar.com",
    "id": "123user",
    "name": "foo"
  }
}
```

```json
{
  "@timestamp": "2017-05-30T18:53:27.154Z",
  "agent": {
    "name": "elastic-node",
    "version": "3.14.0"
  },
  "ecs": {
    "version": "1.6.0"
  },
  "event": {
    "outcome": "unknown"
  },
  "labels": {
    "span_tag": "something"
  },
  "observer": {
    "ephemeral_id": "c0cea3b6-97d7-4e15-9e35-c868e7a3c869",
    "hostname": "ix.lan",
    "id": "a49b4a08-689a-4724-8050-8bd0ae043281",
    "type": "apm-server",
    "version": "8.0.0",
    "version_major": 8
  },
  "parent": {
    "id": "945254c567a5417e"
  },
  "processor": {
    "event": "span",
    "name": "transaction"
  },
  "service": {
    "environment": "staging",
    "name": "1234_service-12a3"
  },
  "span": {
    "action": "query",
    "db": {
      "instance": "customers",
      "statement": "SELECT * FROM product_types WHERE user_id=?",
      "type": "sql",
      "user": {
        "name": "readonly_user"
      }
    },
    "duration": {
      "us": 3781
    },
    "http": {
      "method": "GET",
      "response": {
        "status_code": 200
      },
      "url": {
        "original": "http://localhost:8000"
      }
    },
    "id": "0aaaaaaaaaaaaaaa",
    "name": "SELECT FROM product_types",
    "stacktrace": [
      {
        "abs_path": "net.js",
        "context": {
          "post": [
            "    ins.currentTransaction = prev",
            "    return result",
            "}"
          ],
          "pre": [
            "  var trans = this.currentTransaction",
            ""
          ]
        },
        "exclude_from_grouping": false,
        "filename": "net.js",
        "function": "onread",
        "library_frame": true,
        "line": {
          "column": 4,
          "context": "line3",
          "number": 547
        },
        "module": "some module",
        "vars": {
          "key": "value"
        }
      },
      {
        "exclude_from_grouping": false,
        "filename": "my2file.js",
        "line": {
          "number": 10
        }
      }
    ],
    "start": {
      "us": 2830
    },
    "subtype": "postgresql",
    "sync": false,
    "type": "db"
  },
  "timestamp": {
    "us": 1496170407154000
  },
  "trace": {
    "id": "945254c567a5417eaaaaaaaaaaaaaaaa"
  },
  "transaction": {
    "id": "945254c567a5417e"
  }
}
```


## Metrics

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
|processor.name|Processor name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.event|Processor event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|timestamp.us|Timestamp of the event in microseconds since Unix epoch.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|labels|A flat mapping of user-defined labels with string, boolean or number values.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.name|Immutable name of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.version|Version of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.environment|Service environment.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.node.name|Unique meaningful name of the service node.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.language.name|Name of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.language.version|Version of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.name|Name of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.version|Version of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.name|Name of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.version|Version of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.id|The transaction ID.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|transaction.sampled|Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.type|Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.name|Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id').|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.breakdown.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.root|Identifies metrics for root transactions. This can be used for calculating metrics for traces.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.type|Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', 'cache', etc).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.subtype|A further sub-division of the type (e.g. postgresql, elasticsearch)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|span.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|agent.name|Name of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.version|Version of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.ephemeral_id|The Ephemeral ID identifies a running process.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|container.id|Unique container id.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|kubernetes.namespace|Kubernetes namespace|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.node.name|Kubernetes node name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.name|Kubernetes pod name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.uid|Kubernetes Pod UID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|host.architecture|The architecture of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.hostname|The hostname of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.name|Name of the host the event was recorded on. It can contain same information as host.hostname or a name specified by the user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.ip|IP of the host that records the event.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.os.platform|The platform of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.args|Process arguments. May be filtered to protect sensitive information.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.pid|Numeric process ID of the service process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.ppid|Numeric ID of the service's parent process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.title|Service process title.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.listening|Address the server is listening on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.hostname|Hostname of the APM Server.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version|APM Server version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version_major|Major version number of the observer|byte|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.type|The type will be set to `apm-server`.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.name|The username of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.id|Identifier of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.email|Email of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|client.ip|IP address of the client of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|source.ip|IP address of the source of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.address|Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.ip|IP addess of the destination. Can be one of multiple IPv4 or IPv6 addresses.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.port|Port of the destination.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.original|Unparsed version of the user_agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.name|Name of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.version|Version of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.device.name|Name of the device.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.platform|Operating system platform (such centos, ubuntu, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.name|Operating system name, without the version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.full|Operating system name, including the version or code name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.family|OS family (such as redhat, debian, freebsd, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.version|Operating system version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.kernel|Operating system kernel version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|experimental|Additional experimental data sent by the agents.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|cloud.account.id|Cloud account ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.account.name|Cloud account name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.availability_zone|Cloud availability zone name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.id|Cloud instance/machine ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.name|Cloud instance/machine name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.machine.type|Cloud instance/machine type|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.id|Cloud project ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.name|Cloud project name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.provider|Cloud provider name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.region|Cloud region name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|event.outcome|`event.outcome` simply denotes whether the event represents a success or a failure from the perspective of the entity that produced the event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|system.cpu.total.norm.pct|The percentage of CPU time spent by the process since the last event. This value is normalized by the number of CPU cores and it ranges from 0 to 100%.|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.memory.total|Total memory.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.memory.actual.free|Actual free memory in bytes. It is calculated based on the OS. On Linux it consists of the free memory plus caches and buffers. On OSX it is a sum of free memory and the inactive memory. On Windows, it is equal to `system.memory.free`.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.process.cpu.total.norm.pct|The percentage of CPU time spent by the process since the last event. This value is normalized by the number of CPU cores and it ranges from 0 to 100%.|scaled_float|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.process.memory.size|The total virtual memory the process has.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.process.memory.rss.bytes|The Resident Set Size. The amount of memory the process occupied in main memory (RAM).|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.process.cgroup.memory.mem.limit.bytes|Memory limit for the current cgroup slice.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|system.process.cgroup.memory.mem.usage.bytes|Memory usage by the current cgroup slice.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.name|Processor name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.event|Processor event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|timestamp.us|Timestamp of the event in microseconds since Unix epoch.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|labels|A flat mapping of user-defined labels with string, boolean or number values.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.name|Immutable name of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.version|Version of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.environment|Service environment.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.node.name|Unique meaningful name of the service node.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.language.name|Name of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.language.version|Version of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.name|Name of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.version|Version of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.name|Name of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.version|Version of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|agent.name|Name of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.version|Version of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.ephemeral_id|The Ephemeral ID identifies a running process.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|container.id|Unique container id.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|kubernetes.namespace|Kubernetes namespace|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.node.name|Kubernetes node name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.name|Kubernetes pod name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.uid|Kubernetes Pod UID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|host.architecture|The architecture of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.hostname|The hostname of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.name|Name of the host the event was recorded on. It can contain same information as host.hostname or a name specified by the user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.ip|IP of the host that records the event.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.os.platform|The platform of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.args|Process arguments. May be filtered to protect sensitive information.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.pid|Numeric process ID of the service process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.ppid|Numeric ID of the service's parent process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.title|Service process title.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.listening|Address the server is listening on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.hostname|Hostname of the APM Server.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version|APM Server version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version_major|Major version number of the observer|byte|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.type|The type will be set to `apm-server`.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.name|The username of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.id|Identifier of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.email|Email of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|client.ip|IP address of the client of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|source.ip|IP address of the source of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.address|Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.ip|IP addess of the destination. Can be one of multiple IPv4 or IPv6 addresses.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.port|Port of the destination.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.original|Unparsed version of the user_agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.name|Name of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.version|Version of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.device.name|Name of the device.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.platform|Operating system platform (such centos, ubuntu, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.name|Operating system name, without the version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.full|Operating system name, including the version or code name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.family|OS family (such as redhat, debian, freebsd, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.version|Operating system version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.kernel|Operating system kernel version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|experimental|Additional experimental data sent by the agents.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|cloud.account.id|Cloud account ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.account.name|Cloud account name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.availability_zone|Cloud availability zone name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.id|Cloud instance/machine ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.name|Cloud instance/machine name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.machine.type|Cloud instance/machine type|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.id|Cloud project ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.name|Cloud project name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.provider|Cloud provider name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.region|Cloud region name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|profile.id|Unique ID for the profile. All samples within a profile will have the same profile ID.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.duration|Duration of the profile, in microseconds. All samples within a profile will have the same duration. To aggregate durations, you should first group by the profile ID.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.cpu.ns|Amount of CPU time profiled, in nanoseconds.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.samples.count|Number of profile samples for the profiling period.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.alloc_objects.count|Number of objects allocated since the process started.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.alloc_space.bytes|Amount of memory allocated, in bytes, since the process started.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.inuse_objects.count|Number of objects allocated and currently in use.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.inuse_space.bytes|Amount of memory allocated, in bytes, and currently in use.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.top.id|Unique ID for the top stack frame in the context of its callers.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.top.function|Function name for the top stack frame.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.top.filename|Source code filename for the top stack frame.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.top.line|Source code line number for the top stack frame.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.stack.id|Unique ID for a stack frame in the context of its callers.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.stack.function|Function name for a stack frame.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.stack.filename|Source code filename for a stack frame.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|profile.stack.line|Source code line number for a stack frame.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |


### Example

```json
{
  "@timestamp": "2017-05-30T18:53:41.364Z",
  "agent": {
    "name": "elastic-node",
    "version": "3.14.0"
  },
  "ecs": {
    "version": "1.6.0"
  },
  "event": {
    "ingested": "2020-04-22T14:55:05.425020Z"
  },
  "go": {
    "memstats": {
      "heap": {
        "sys": {
          "bytes": 6520832
        }
      }
    }
  },
  "host": {
    "ip": "127.0.0.1"
  },
  "labels": {
    "tag1": "one",
    "tag2": 2
  },
  "observer": {
    "ephemeral_id": "8785cbe1-7f89-4279-84c2-6c33979531fb",
    "hostname": "ix.lan",
    "id": "b0cfe4b7-76c9-4159-95ff-e558db368cbe",
    "type": "apm-server",
    "version": "8.0.0",
    "version_major": 8
  },
  "process": {
    "pid": 1234
  },
  "processor": {
    "event": "metric",
    "name": "metric"
  },
  "service": {
    "language": {
      "name": "ecmascript"
    },
    "name": "1234_service-12a3",
    "node": {
      "name": "node-1"
    }
  },
  "user": {
    "email": "user@mail.com",
    "id": "axb123hg",
    "name": "logged-in-user"
  }
}
```

## Logs

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
|processor.name|Processor name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|processor.event|Processor event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|timestamp.us|Timestamp of the event in microseconds since Unix epoch.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|url.scheme|The protocol of the request, e.g. "https:".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.full|The full, possibly agent-assembled URL of the request, e.g https://example.com:443/search?q=elasticsearch#top.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.domain|The hostname of the request, e.g. "example.com".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.port|The port of the request, e.g. 443.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.path|The path of the request, e.g. "/search".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.query|The query string of the request, e.g. "q=elasticsearch".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|url.fragment|A fragment specifying a location in a web page , e.g. "top".|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.version|The http version of the request leading to this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.request.method|The http method of the request leading to this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.request.headers|The canonical headers of the monitored HTTP request.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|http.request.referrer|Referrer for this HTTP request.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.response.status_code|The status code of the HTTP response.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|http.response.finished|Used by the Node agent to indicate when in the response life cycle an error has occurred.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|http.response.headers|The canonical headers of the monitored HTTP response.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|labels|A flat mapping of user-defined labels with string, boolean or number values.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.name|Immutable name of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.version|Version of the service emitting this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.environment|Service environment.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.node.name|Unique meaningful name of the service node.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|service.language.name|Name of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.language.version|Version of the programming language used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.name|Name of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.runtime.version|Version of the runtime used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.name|Name of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|service.framework.version|Version of the framework used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.id|The transaction ID.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|transaction.sampled|Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have spans or context.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.type|Keyword of specific relevance in the service's domain (eg. 'request', 'backgroundjob', etc)|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.name|Generic designation of a transaction in the scope of a single service (eg. 'GET /users/:id').|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.duration.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.self_time.sum.us||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|transaction.breakdown.count||long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|trace.id|The ID of the trace to which the event belongs to.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|parent.id|The ID of the parent event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|agent.name|Name of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.version|Version of the agent used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|agent.ephemeral_id|The Ephemeral ID identifies a running process.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|container.id|Unique container id.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|kubernetes.namespace|Kubernetes namespace|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.node.name|Kubernetes node name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.name|Kubernetes pod name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|kubernetes.pod.uid|Kubernetes Pod UID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|host.architecture|The architecture of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.hostname|The hostname of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.name|Name of the host the event was recorded on. It can contain same information as host.hostname or a name specified by the user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.ip|IP of the host that records the event.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|host.os.platform|The platform of the host the event was recorded on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.args|Process arguments. May be filtered to protect sensitive information.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.pid|Numeric process ID of the service process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.ppid|Numeric ID of the service's parent process.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|process.title|Service process title.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.listening|Address the server is listening on.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.hostname|Hostname of the APM Server.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version|APM Server version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|observer.version_major|Major version number of the observer|byte|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|observer.type|The type will be set to `apm-server`.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.name|The username of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.id|Identifier of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user.email|Email of the logged in user.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|client.ip|IP address of the client of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|source.ip|IP address of the source of a recorded event. This is typically obtained from a request's X-Forwarded-For or the X-Real-IP header or falls back to a given configuration for remote address.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.address|Some event destination addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.ip|IP addess of the destination. Can be one of multiple IPv4 or IPv6 addresses.|ip|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|destination.port|Port of the destination.|long|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.original|Unparsed version of the user_agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.name|Name of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.version|Version of the user agent.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.device.name|Name of the device.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.platform|Operating system platform (such centos, ubuntu, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.name|Operating system name, without the version.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.full|Operating system name, including the version or code name.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.family|OS family (such as redhat, debian, freebsd, windows).|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.version|Operating system version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|user_agent.os.kernel|Operating system kernel version as a raw string.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|experimental|Additional experimental data sent by the agents.|object|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|cloud.account.id|Cloud account ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.account.name|Cloud account name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.availability_zone|Cloud availability zone name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.id|Cloud instance/machine ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.instance.name|Cloud instance/machine name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.machine.type|Cloud instance/machine type|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.id|Cloud project ID|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.project.name|Cloud project name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.provider|Cloud provider name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|cloud.region|Cloud region name|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|error.id|The ID of the error.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png)  |
|error.culprit|Function call which was the primary perpetrator of this event.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.grouping_key|GroupingKey of the logged error for use in grouping.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.exception.code|The error code set when the error happened, e.g. database error code.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.exception.message|The original error message.|text|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.exception.module|The module namespace of the original error.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.exception.type||keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.exception.handled|Indicator whether the error was caught somewhere in the code or not.|boolean|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.log.level|The severity of the record.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.log.logger_name|The name of the logger instance used.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.log.message|The additionally logged error message.|text|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |
|error.log.param_message|A parametrized message. E.g. 'Could not connect to %s'. The property message is still required, and should be equal to the param_message, but with placeholders replaced. In some situations the param_message is used to group errors together.|keyword|  ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png)  |


### Example

```json
{
  "@timestamp": "2017-05-09T15:04:05.999Z",
  "agent": {
    "name": "elastic-node",
    "version": "3.14.0"
  },
  "container": {
    "id": "container-id"
  },
  "ecs": {
    "version": "1.6.0"
  },
  "error": {
    "grouping_key": "d6b3f958dfea98dc9ed2b57d5f0c48bb",
    "id": "0f0e9d67c1854d21a6f44673ed561ec8",
    "log": {
      "level": "custom log level",
      "message": "Cannot read property 'baz' of undefined"
    }
  },
  "event": {
    "ingested": "2020-04-22T14:52:08.436124Z"
  },
  "host": {
    "architecture": "x64",
    "ip": "127.0.0.1",
    "os": {
      "platform": "darwin"
    }
  },
  "kubernetes": {
    "namespace": "namespace1",
    "pod": {
      "name": "pod-name",
      "uid": "pod-uid"
    }
  },
  "labels": {
    "tag1": "one",
    "tag2": 2
  },
  "observer": {
    "ephemeral_id": "f1838cde-80dd-4af5-b7ac-ffc2d3fccc9d",
    "hostname": "ix.lan",
    "id": "5d4dc8fe-cb14-47ee-b720-d6bf49f87ef0",
    "type": "apm-server",
    "version": "8.0.0",
    "version_major": 8
  },
  "process": {
    "args": [
      "node",
      "server.js"
    ],
    "pid": 1234,
    "ppid": 7788,
    "title": "node"
  },
  "processor": {
    "event": "error",
    "name": "error"
  },
  "service": {
    "environment": "staging",
    "framework": {
      "name": "Express",
      "version": "1.2.3"
    },
    "language": {
      "name": "ecmascript",
      "version": "8"
    },
    "name": "1234_service-12a3",
    "node": {
      "name": "myservice-node"
    },
    "runtime": {
      "name": "node",
      "version": "8.0.0"
    },
    "version": "5.1.3"
  },
  "timestamp": {
    "us": 1494342245999000
  }
}
```
