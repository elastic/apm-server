#!/usr/bin/env python

from __future__ import print_function
import os
import shutil
import argparse


def is_dir(d):
    if os.path.isdir(d) and os.access(d, os.R_OK):
        return d
    raise Exception("Invalid directory")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-t', '--target_dir', type=is_dir, default='docs/copied-from-beats')
    parser.add_argument('-s', '--source_dir', type=is_dir, default='../beats/libbeat/docs')
    args = parser.parse_args()

    start = len(args.target_dir.strip("/"))+1
    for root, subdirs, files in os.walk(args.target_dir):
        for fname in files:
            target = os.path.join(root, fname)
            source = os.path.join(args.source_dir, os.path.join(root[start::], fname))
            try:
                shutil.copyfile(source, target)
            except:
                print("File doesn't exist: {}".format(source))


if __name__ == '__main__':
    main()
