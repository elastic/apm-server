# APM Rally Track

For general information about rally tracks and how to use them, 
check out the [docs](https://esrally.readthedocs.io/en/stable/index.html).
and the [rally](https://github.com/elastic/rally) and [rally-tracks](https://github.com/elastic/rally-tracks) github 
repos.

## Corpora
Data used in this track are dumped from example Opbeans applications, instrumented with `nodejs`, `python` and 
`ruby` agents. The dump was created in September 2018. 
You can also build your own corpora and reference them in the track.
There is a [python script](https://github.com/elastic/apm-server/blob/d7b9d5027dd6a296792aa5179c0eaff8374d62d8/rally/fetch_data.py).
you can use for fetching data from an Elasticsearch instance and bringing them into the expected shape. 

## Starting a Race

At the moment APM track offers two different challenges. 

### Default challenge
Ingest data of type `error,transactions,spans` in parallel into Elasticsearch.
You can start the race for the default challenge with `esrally --track-path=<local-path-to-apm-track>`.

### Single Event Type challenge
If you want to test only a dedicated event type, you can use the second challenge, by running 
`esrally --track-path=<local-path-to-apm-track> --track-params="event_type:'<event_type>'" --challenge=ingest-event-type`


If you want to store results to a file, preserve temporary Elasticsearch, use a running Elasticsearch instance, etc. 
please refer to rally's [command_line_reference](https://esrally.readthedocs.io/en/stable/command_line_reference.html#command-line-flags).


## Parameters
When providing `--track-params` you can override following default parameters for the challenges: 

* ingest_percentage: Percentage defining how much of the document corpus should be indexed. Defaults to 100. 
* bulk-size: Number of documents per bulk operation. Defaults to 1_000.
* bulk_indexing_clients: Number of clients running bulk indexing requests.

Read more about [track-params](https://esrally.readthedocs.io/en/stable/command_line_reference.html#track-params) in general. 
