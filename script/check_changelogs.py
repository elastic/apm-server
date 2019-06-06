#!/Users/gil/.venv/ipy/bin/python

import io
import hashlib
import os

import requests


VERSIONS = ["6.0", "6.1", "6.2", "6.3", "6.4", "6.5", "6.6", "6.7", "6.8", "7.0", "7.1", "7.x"]


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
    for cl in sorted(os.listdir(cl_dir)):
        version, _ = os.path.splitext(cl)
        if version == 'head':
            continue
        parsed_version = parse_version(version)
        with open(os.path.join(cl_dir, cl), mode='rb') as f:
            master = shasum(f)

        print("**", cl, master, "**")
        for v in VERSIONS:
            if parsed_version <= parse_version(v):
                print(f"checking {cl} on {v}")
                url = f"https://raw.githubusercontent.com/elastic/apm-server/{v}/changelogs/{cl}"
                rsp = requests.get(url)
                if rsp.status_code == 200:
                    h = shasum(io.BytesIO(rsp.content))
                else:
                    h = f"error: {rsp.status_code}"
                # rsp.raise_for_status()
                print(h, url, "🔴" if h != master else "✅")
        print()


if __name__ == '__main__':
    main()
