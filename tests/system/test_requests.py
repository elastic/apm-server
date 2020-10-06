from collections import defaultdict
import gzip
import requests
import threading
import time
import zlib
from io import BytesIO

from apmserver import ServerBaseTest, ClientSideBaseTest, CorsBaseTest


class Test(ServerBaseTest):

    def test_ok(self):
        r = self.request_intake()
        assert r.status_code == 202, r.status_code
        assert r.text == "", r.text

    def test_ok_verbose(self):
        r = self.request_intake(url='http://localhost:8200/intake/v2/events?verbose')
        assert r.status_code == 202, r.status_code
        assert r.json() == {"accepted": 4}, r.json()

    def test_empty(self):
        r = self.request_intake(data={})
        assert r.status_code == 400, r.status_code

    def test_not_existent(self):
        r = self.request_intake(url='http://localhost:8200/transactionX')
        assert r.status_code == 404, r.status_code

    def test_method_not_allowed(self):
        r = requests.get(self.intake_url)
        assert r.status_code == 400, r.status_code

    def test_bad_json(self):
        r = self.request_intake(data="invalid content")
        assert r.status_code == 400, r.status_code

    def test_validation_fail(self):
        data = self.get_event_payload(name="invalid-event.ndjson")
        r = self.request_intake(data=data)
        assert r.status_code == 400, r.status_code
        assert "decode error: data read error" in r.text, r.text

    def test_rum_default_disabled(self):
        r = self.request_intake(url='http://localhost:8200/intake/v2/rum/events')
        assert r.status_code == 403, r.status_code

    def test_healthcheck(self):
        healtcheck_url = 'http://localhost:8200/'
        r = requests.get(healtcheck_url)
        assert r.status_code == 200, r.status_code

    def test_gzip(self):
        events = self.get_event_payload().encode("utf-8")
        out = BytesIO()

        with gzip.GzipFile(fileobj=out, mode="w") as f:
            f.write(events)

        r = requests.post(self.intake_url, data=out.getvalue(),
                          headers={'Content-Encoding': 'gzip', 'Content-Type': 'application/x-ndjson'})
        assert r.status_code == 202, r.status_code

    def test_deflate(self):
        events = self.get_event_payload().encode("utf-8")
        compressed_data = zlib.compress(events)

        r = requests.post(self.intake_url, data=compressed_data,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/x-ndjson'})
        assert r.status_code == 202, r.status_code

    def test_gzip_error(self):
        events = self.get_event_payload()
        r = requests.post(self.intake_url, json=events,
                          headers={'Content-Encoding': 'gzip', 'Content-Type': 'application/x-ndjson'})
        assert r.status_code == 400, r.status_code

    def test_deflate_error(self):
        events = self.get_event_payload()
        r = requests.post(self.intake_url, data=events,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/x-ndjson'})
        assert r.status_code == 400, r.status_code

    def test_expvar_default(self):
        """expvar should not be exposed by default"""
        r = requests.get(self.expvar_url)
        assert r.status_code == 404, r.status_code


class ClientSideTest(ClientSideBaseTest):

    def test_ok(self):
        r = self.request_intake()
        assert r.status_code == 202, r.status_code

    def test_sourcemap_upload_fail(self):
        path = self._beat_path_join(
            'testdata',
            'sourcemap',
            'bundle.js.map')
        file = open(path)
        r = requests.post(self.sourcemap_url,
                          files={'sourcemap': file})
        assert r.status_code == 400, r.status_code


class CorsTest(CorsBaseTest):

    def test_ok(self):
        r = self.request_intake(headers={'Origin': 'http://www.elastic.co', 'content-type': 'application/x-ndjson'})
        assert r.headers['Access-Control-Allow-Origin'] == 'http://www.elastic.co', r.headers
        assert r.status_code == 202, r.status_code

    def test_bad_origin(self):
        # origin must include protocol and match exactly the allowed origin
        r = self.request_intake(headers={'Origin': 'www.elastic.co', 'content-type': 'application/x-ndjson'})
        assert r.status_code == 403, r.status_code

    def test_no_origin(self):
        r = self.request_intake()
        assert r.status_code == 403, r.status_code

    def test_preflight(self):
        r = requests.options(self.intake_url,
                             data=self.get_event_payload(),
                             headers={'Origin': 'http://www.elastic.co',
                                      'Access-Control-Request-Method': 'POST',
                                      'Access-Control-Request-Headers': 'Content-Type, Content-Encoding'})
        assert r.status_code == 200, r.status_code
        assert r.headers['Access-Control-Allow-Origin'] == 'http://www.elastic.co', r.headers
        assert r.headers['Access-Control-Allow-Headers'] == 'Content-Type, Content-Encoding, Accept', r.headers
        assert r.headers['Access-Control-Allow-Methods'] == 'POST, OPTIONS', r.headers
        assert r.headers['Vary'] == 'Origin', r.headers
        assert r.headers['Content-Length'] == '0', r.headers
        assert r.headers['Access-Control-Max-Age'] == '3600', r.headers

    def test_preflight_bad_headers(self):
        for h in [{'Access-Control-Request-Method': 'POST'}, {'Origin': 'www.elastic.co'}]:
            r = requests.options(self.intake_url,
                                 json=self.get_event_payload(),
                                 headers=h)
            assert r.status_code == 200, r.status_code
            assert 'Access-Control-Allow-Origin' not in r.headers.keys(), r.headers
            assert r.headers['Access-Control-Allow-Headers'] == 'Content-Type, Content-Encoding, Accept', r.headers
            assert r.headers['Access-Control-Allow-Methods'] == 'POST, OPTIONS', r.headers


class RateLimitTest(ClientSideBaseTest):

    def fire_events(self, data_file, iterations, split_ips=False):
        events = self.get_event_payload(name=data_file)
        threads = []
        codes = defaultdict(int)

        def fire(x):
            ip = '10.11.12.13'
            if split_ips and x % 2:
                ip = '10.11.12.14'
            r = self.request_intake(data=events,
                                    headers={'content-type': 'application/x-ndjson', 'X-Forwarded-For': ip})
            codes[r.status_code] += 1
            return r.status_code

        # rate limit hit, because every event in request is counted
        for x in range(iterations):
            threads.append(threading.Thread(target=fire, args=(x,)))

        for t in threads:
            t.start()
            time.sleep(0.01)

        for t in threads:
            t.join()
        return codes

    # limit: 16, burst_multiplier: 3, burst: 48
    def test_rate_limit(self):
        # all requests from the same ip
        # 19 events, batch size 10 => 20+1 events per requ
        codes = self.fire_events("ratelimit.ndjson", 3)
        assert set(codes.keys()) == set([202]), codes

    def test_rate_limit_hit(self):
        # all requests from the same ip
        codes = self.fire_events("ratelimit.ndjson", 5)
        assert set(codes.keys()) == set([202, 429]), codes
        assert codes[429] == 2, codes
        assert codes[202] == 3, codes

    def test_rate_limit_small_hit(self):
        # all requests from the same ip
        # 4 events, batch size 10 => 10+1 events per requ
        codes = self.fire_events("events.ndjson", 8)
        assert set(codes.keys()) == set([202, 429]), codes
        assert codes[429] == 3, codes
        assert codes[202] == 5, codes

    def test_rate_limit_only_metadata(self):
        # all requests from the same ip
        # no events, batch size 10 => 10+1 events per requ
        codes = self.fire_events("metadata.ndjson", 8)
        assert set(codes.keys()) == set([202, 429]), codes
        assert codes[429] == 3, codes
        assert codes[202] == 5, codes

    def test_multiple_ips_rate_limit(self):
        # requests from 2 different ips
        codes = self.fire_events("ratelimit.ndjson", 6, True)
        assert set(codes.keys()) == set([202]), codes

    def test_multiple_ips_rate_limit_hit(self):
        # requests from 2 different ips
        codes = self.fire_events("ratelimit.ndjson", 10, True)
        assert set(codes.keys()) == set([202, 429]), codes
        assert codes[429] == 4, codes
        assert codes[202] == 6, codes
