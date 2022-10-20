# Architecture of the APM Server

This document gives a high level overview over the architecture of the main APM Server components.
The main purpose of the APM Server is to ingest data. It validates, enriches and transforms data,
received from a variety of APM agents, into a dedicated format and passes them on to an output,
such as Elasticsearch.

## Ingest Flow

High level overview over incoming data and their flow through the APM Server until being passed on
to the output publisher pipeline.

![ingest-flow](../docs/images/ingest-flow.png)

## Modelindexer Architecture

When APM Server uses a custom Elasticsearch output called `modelindexer`. It fills a local cache until it
is full, and then, flushes the cache in the background and continues processing events.

From  `8.0` until `8.5`, the _modelindexer_ processed the events synchronously and used mutexes for
synchronized writes to the cache. This worked well, but didn't seem to scale well on bigger instances with
more CPUs.

```mermaid
flowchart LR;
    subgraph Goroutine
        Flush;
    end
    AgentA & AgentB-->Handler;
    subgraph Intake
    Handler<-->|semaphore|Decode
    Decode-->Batch;
    end
    subgraph ModelIndexer
    Available-.->Active;
    Batch-->Active;
    Active<-->|mutex|Cache;
    end
    Cache-->|FullOrTimer|Flush;

    Flush-->|bulk|ES[(Elasticsearch)];
    Flush-->|done|Available;
```

From `8.6.0` onwards, the _modelindexer_ accepts events asynchronously and runs one or more _active indexers_,
which pull events from a local queue and (by default) compress them and write them to the local cache. This approach
has reduced locking, and the number of active indexers is automatically scaled up and down based on how full the
outgoing flushes are, with a hard limit on the number of indexers depending on the hardware.

```mermaid
flowchart LR;
    subgraph Goroutine11
        Flush1(Flush);
    end
    subgraph Goroutine22
        Flush2(Flush);
    end
    AgentA & AgentB-->Handler;
    subgraph Intake
    Handler<-->|semaphore|Decode
    Decode-->Batch;
    end
    subgraph ModelIndexer
    Batch-->Buffer;
    Available;
        subgraph Goroutine1
            Active1(Active);
            Active1(Active)<-->Cache1(Cache);
            Cache1(Cache)-->|FullOrTimer|Flush1(Flush);
        end
        subgraph Goroutine2
            Active2(Active);
            Active2(Active)<-->Cache2(Cache);
            Cache2(Cache)-->|FullOrTimer|Flush2(Flush);
        end
        subgraph Channel
            Buffer-->Active1(Active) & Active2(Active);
        end
        Available-.->Active1(Active) & Active2(Active);
    end

    Flush1(Flush) & Flush2(Flush)-->|bulk|ES[(Elasticsearch)];
    Flush1(Flush) & Flush2(Flush)-->|done|Available;
```
