'use strict'

var apm = require('elastic-apm-node').start({
  appName: 'test-app',
  maxQueueSize: 1
})


var app = require("express")();

app.get("/", function(req, res) {
    res.send("OK");
});

app.get("/foo", function(req, res) {
    foo_route()
    res.send("OK");
});

function foo_route () {
    var trace = apm.buildTrace()
    trace.start('app.foo')
    trace.end()
}

app.get("/bar", function(req, res) {
    bar_route()
    res.send("OK");
});

function bar_route () {
    var trace = apm.buildTrace()
    trace.start('app.bar')
    trace.end()
}


app.use(apm.middleware.express())

var server = app.listen(8081, function () {
    console.log("Listening on %s...", server.address().port);
});

