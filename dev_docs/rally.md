# Rally: Elasticsearch performance benchmarking

:construction: Work in progress :construction:

As we make changes to the way APM Server indexes documents,
we may alter the performance of indexing or queries, and we
may alter our storage efficiency. We can use [Rally][rally]
for benchmarking these.

## Benchmarking APM & Elasticsearch

Rally was designed with a focus on benchmarking Elasticsearch:
given a set of documents, what is the performance of indexing?
Querying? Deleting? And so on. Typically one will produce a
corpus (set of documents), and then run the benchmark with
varying versions of Elasticsearch, or on varying hardware
profiles. This is almost what we want.

Like Elasticsearch, the Elastic APM product evolves over time.
In practice, that means:

 - documents produce by APM Server change shape
 - processing moves from APM Server to ingest node (and vice versa)
 - queries made by APM UI change

If we were to benchmark with a fixed Elasticsearch document
corpus over time, we would not be able to measure the impact
of the first two points. Therefore we use Rally in a slightly
unconventional way.

Instead of storing fixed Elasticsearch document corpora, we
instead maintain fixed corpora of APM events. We translate these
at benchmark time to the current document structure, using the
APM Server binary. This can be achieved with `make rally/corpora`.
We install the current APM integration package via Fleet, to ensure
the current index templates and ingest pipelines are defined.
We then run the corpora through Rally as usual, with `make rally`.

This tells us little about the change in processing performance
of APM Server, or the overall ingest performance. For example if
we move processing from APM Server to ingest node, we may identify
an increase in Elasticsearch processing time, but this may be
offset by a decrease in APM Server processing time which we will
not observe. We have separate tooling (apmbench) for macrobenchmarking
the entire ingest process, from agent to APM Server to Elasticsearch.
Our use of Rally is purely focused on analysing Elasticsearch ingest
performance, query performance, and storage efficiency.

[rally]: https://github.com/elastic/rally
