from functools import cmp_to_key

import argparse
import os
import shutil
import subprocess
import sys


def semver_sorter(a, b):
    a_list = a.split("-")
    b_list = b.split("-")
    version_cmp = trivial_cmp(a_list[0], b_list[0])
    if version_cmp != 0:
        return version_cmp
    if len(a_list) == 1:
        return 1
    if len(b_list) == 1:
        return -1
    return trivial_cmp(a_list[1], b_list[1])


def trivial_cmp(a, b):
    if a > b:
        return 1
    elif b > a:
        return -1
    return 0


def bump(v):
    tokens = v.split(".")
    tokens[-1] = str(int(tokens[-1]) + 1)
    return ".".join(tokens)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--dst', help='directory of the package-storage repo', default="../../package-storage")
    parser.add_argument('--final', action='store_true')
    parser.add_argument('-v', '--version', help='version of the package to copy, defaults to last one')
    parser.add_argument('--dry', action='store_true', help='dont copy data')
    args = parser.parse_args()

    src = "../apmpackage/apm/"
    original_version = args.version
    if not args.version:
        # default to last version
        versions = [os.path.basename(f) for f in os.listdir(src)]
        versions.sort()
        original_version = versions[-1]

    src = os.path.join(src, original_version)
    dst = os.path.join(args.dst, "packages/apm/")

    # find and sort published versions
    published_versions = [os.path.basename(f) for f in os.listdir(dst)]
    published_versions.sort(key=cmp_to_key(semver_sorter))
    published_versions.reverse()

    # resolve the next version
    # only patch and dev might be automatically bumped
    next_version = original_version
    for published in published_versions:
        if original_version == published:
            raise Exception("Version already published")
        if original_version in published:
            if not args.final:
                # development version released, bump it
                # eg. 0.1.0-dev.3 -> 0.1.0-dev.4
                next_version = bump(published)
            break

    if next_version == original_version and not args.final:
        # version never released, create the first development version out of it
        next_version = next_version + "-dev.1"

    dst = os.path.join(dst, next_version)
    print("from " + src + " to " + dst)

    if args.dry:
        sys.exit(0)

    # copy over the package and replace version in manifest and pipeline names
    shutil.copytree(src, dst)
    subprocess.check_call('rm -rf {0}'.format(os.path.join(dst, 'README.template.md')), shell=True)
    cmd = 'find {0} -not -name "*.png" -type f -print0 | xargs -0 sed -i "" "s/{1}/{2}/g"'.format(
        dst, original_version, next_version)

    out = subprocess.check_output(cmd, shell=True)
    if out:
        print(out)
    print("Done")
