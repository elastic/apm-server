# Architecture of the APM Server

This document gives a high level overview over the architecture of the main APM Server components.
The main purpose of the APM Server is to ingest data. It validates, enriches and transforms data, that it receives from a variety of APM agents, into a dedicated format and passes them on to an output, such as Elasticsearch. 

## Ingest Flow
High level overview over incoming data and their flow through the APM Server until being passed on to the output publisher pipeline. 

![](./docs/images/ingest-flow.png)