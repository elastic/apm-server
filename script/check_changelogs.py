#!/usr/bin/env python3

import io
import hashlib
import os
import requests

SUPPORTED_VERSIONS = ["6.8", "7.8", "7.x"]


def parse_version(version):
    return tuple([int(x) if x != "x" else 100 for x in version.split('.')])


def shasum(fp):
    h = hashlib.sha1()
    while True:
        buf = fp.read()
        if len(buf) == 0:
            break
        h.update(buf)
    return h.hexdigest()


def main():

    cl_dir = 'changelogs'
    any_failures = False
    for cl in sorted(os.listdir(cl_dir)):
        version, _ = os.path.splitext(cl)
        if version not in SUPPORTED_VERSIONS:
            continue
        parsed_version = parse_version(version)
        with open(os.path.join(cl_dir, cl), mode='rb') as f:
            master = shasum(f)

        print("**", cl, master, "**")
        for v in SUPPORTED_VERSIONS:
            if parsed_version <= parse_version(v):
                print("checking {} on {}".format(cl, v))
                url = "https://raw.githubusercontent.com/elastic/apm-server/{}/changelogs/{}".format(v, cl)
                rsp = requests.get(url)
                status = "success"
                if rsp.status_code == 200:
                    h = shasum(io.BytesIO(rsp.content))
                else:
                    h = "error: {}".format(rsp.status_code)
                # rsp.raise_for_status()
                if h != master:
                    status = "failed"
                    any_failures = True
                print(h, url, status)
        print()
    if any_failures:
        raise Exception('Some changelogs are missing, please look at for failed.')


if __name__ == '__main__':
    main()
