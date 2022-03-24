# Intake Receiver

Intake Receiver is a tool based on the APM Server Intake protocol, which
will listen for events to be sent to the socket it's listening to, and will
store the events in new line delimitted JSON files, one per `<agent>-<version>.ndjson`.

This tool is used for active benchmarking of the APM Server.

This is not an official product, and comes with no warranty or support.

## Usage

```console
$ ./intake-receiver -h
  -folder string
    	The path where the received intake events will be stored (default "events")
  -host string
    	port that the server will listen to (default ":8200")
```

```console
$ ./intake-receiver
2022/02/24 20:02:17 http server listening for requests on :8200
2022/02/24 20:02:57 processed request to /intake/v2/events (elasticapm-go/1.15.0 go/go1.17.7) in 370.917µs
2022/02/24 20:02:57 processed request to /intake/v2/events (elasticapm-go/1.15.0 go/go1.17.7) in 195.458µs
2022/02/24 20:02:57 processed request to /intake/v2/events (elasticapm-go/1.15.0 go/go1.17.7) in 212.292µs
^C2022/02/24 20:03:52 closing http server...
2022/02/24 20:03:52 closed file events/go-1.15.0.ndjson
2022/02/24 20:03:52 closed file events/go-2.0.0.ndjson
2022/02/24 20:03:52 closed file events/ruby-4.5.0.ndjson
2022/02/24 20:03:52 closed file events/python-6.7.2.ndjson
```
