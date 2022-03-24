#!/usr/bin/env python3
#
# This tool runs apm-server with the Elasticsearch output configured to
# point at a fake Elasticsearch server which writes out the request body
# of _bulk requests. This can be used for capturing Elasticsearch documents
# produced by apm-server, e.g. for testing index mapping or Ingest pipeline
# changes with Rally.

import http.server
import io
import json
import os.path
import signal
import subprocess
import sys
import threading
import gzip


class BulkHandler(http.server.BaseHTTPRequestHandler):
    def log_request(code='-', size='-'):
        pass

    def do_GET(self):
        self.send_response(200)
        self.send_header("X-Elastic-Product", "Elasticsearch")
        self.end_headers()
        self.wfile.write(b'{"cluster_uuid":"cat_bulk"}')

    def do_POST(self):
        content_length = int(self.headers["Content-Length"])
        content = self.rfile.read(content_length)

        encoding = self.headers.get("Content-Encoding", None)
        if encoding == "gzip":
            content = gzip.decompress(content)

        items = []
        for i, line in enumerate(io.BytesIO(content)):
            sys.stdout.buffer.write(line)
            item = json.loads(line)
            if i % 2 == 0:
                action = list(item.keys())[0]
                items.append({action: {"status": 200}})
        encoded = json.dumps({"items": items})
        self.send_response(200)
        self.send_header("X-Elastic-Product", "Elasticsearch")
        self.end_headers()
        self.wfile.write(encoded.encode("utf-8"))


if __name__ == "__main__":
    script_dir = os.path.dirname(sys.argv[0])
    root_dir = os.path.join(script_dir, "..")

    httpd = http.server.HTTPServer(("127.0.0.1", 0), BulkHandler)
    proc = subprocess.Popen([
        "./apm-server", "--strict.perms=false",
        "-e", "-E", "logging.level=warning",
        "-E", "apm-server.data_streams.wait_for_integration=false",
        "-E", "output.elasticsearch.hosts=['localhost:{}']".format(httpd.server_port),
    ], cwd=root_dir, stdout=sys.stdout, stderr=sys.stderr)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()

    def on_sigterm(*args):
        proc.send_signal(signal.SIGTERM)
    signal.signal(signal.SIGTERM, on_sigterm)
    proc.wait()
    httpd.shutdown()
