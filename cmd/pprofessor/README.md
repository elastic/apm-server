## pprofessor

pprofessor is based on the [pprof](https://github.com/google/pprof) tool,
using an alternative "Fetcher" implementation that queries Elasticsearch to
aggregate continuous profiling samples recorded by Elastic APM Server.

This is not an official product, and comes with no warranty or support.

### Usage

```
pprofessor --text -sample_index=cpu -start=now-1h http://admin:changeme@localhost:9200
```

### Dependencies

 - Elasticsearch and Elastic APM Server (same stack version as this branch)
 - Graphviz: http://www.graphviz.org/ Optional, used to generate graphic visualizations of profiles
