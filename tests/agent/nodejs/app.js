'use strict'

var opbeat = require('opbeat').start({
  appName: 'nodejs-testapp',
  secretToken: '123',
  apiHost: 'localhost',
  apiPort: '8080',
  flushInterval: 1,
})

var http = require('http')

http.createServer(function (req, res) {
  console.log(req.method, req.url)

  res.writeHead(200, {'Content-Type': 'text/plain'})
  res.end('Hello World\n')

  opbeat.captureError(new Error('Ups, something broke'))
}).listen(8081, function () {
  console.log('server is listening on port 8081')
})
