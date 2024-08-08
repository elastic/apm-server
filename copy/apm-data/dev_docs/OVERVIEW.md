# Overview

## Directories

- `input/`
  - Struct definitions of input models, e.g. Intake v2, RUM v3, OTel
  - Decoding and translation logic from these input models to the internal model `APMEvent`
- `model/proto`
  - Protobuf definitions of the internal model `APMEvent`
- `model/modelpb`
  - Struct definitions of the internal model `APMEvent`
  - Automatically generated from the protobuf definitions
- `model/internal/modeljson`
  - JSON serialization logic for indexing into Elasticsearch

## Data Flow (APM Server)

1. As an event is sent from an agent to APM server, it is deserialized to an input model (e.g. Intake v2, RUM v3, OTel).
2. The input model will then be decoded and translated to the internal model `APMEvent`.
3. The `APMEvent` is used throughout APM server for all processing and aggregation.
4. The internal model `APMEvent` is converted to modeljson.
4. At last, modeljson will be serialized into JSON and indexed into Elasticsearch.

(All of the above is done in batches.)
