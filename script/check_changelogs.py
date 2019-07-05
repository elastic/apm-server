#!/usr/bin/env python

import io
import hashlib
import os
import argparse
import requests


VERSIONS = ["6.0", "6.1", "6.2", "6.3", "6.4", "6.5", "6.6", "6.7", "6.8", "7.0", "7.1", "7.2", "7.x"]

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

    parser = argparse.ArgumentParser(description=f"Check changelogs for the current released versions. {VERSIONS}")
    parser.add_argument('--fail-if-errors', dest='fail_if_errors', action='store_true', default=False,
                        help='fail the check if there are check errors.')

    args = parser.parse_args()

    cl_dir = 'changelogs'
    for cl in sorted(os.listdir(cl_dir)):
        version, _ = os.path.splitext(cl)
        if version == 'head':
            continue
        parsed_version = parse_version(version)
        with open(os.path.join(cl_dir, cl), mode='rb') as f:
            master = shasum(f)

        any_failures = False
        print("**", cl, master, "**")
        for v in VERSIONS:
            if parsed_version <= parse_version(v):
                print(f"checking {cl} on {v}")
                url = f"https://raw.githubusercontent.com/elastic/apm-server/{v}/changelogs/{cl}"
                rsp = requests.get(url)
                status = "✅"
                if rsp.status_code == 200:
                    h = shasum(io.BytesIO(rsp.content))
                else:
                    h = f"error: {rsp.status_code}"
                # rsp.raise_for_status()
                if h != master:
                    status = "🔴"
                    any_failures = True
                print(h, url, status)
        print()
        if args.fail_if_errors and any_failures:
            raise Exception('Some changelogs are missing, please look at for 🔴.')

if __name__ == '__main__':
    main()
