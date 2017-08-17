'use strict'

var apm = require('elastic-apm').start({
  // Set required app name (allowed characters: a-z, A-Z, 0-9, -, _, and space)
  appName: 'nodejs-testapp',

  // Use if APM Server requires a token
  secretToken: '',

  // Set custom APM Server URL (default: http://localhost:8080)
  serverUrl: '',

  flushInterval: 1
})

var http = require('http')

http.createServer(function (req, res) {
  console.log(req.method, req.url)

  res.writeHead(200, {'Content-Type': 'text/plain'})
  res.end('Hello World\n')

  apm.captureError(new Error('Ups, something broke'))

}).listen(8081, function () {
  console.log('server is listening on port 8081')
})
