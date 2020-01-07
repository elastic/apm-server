#!/usr/bin/env bash

set -e

name=$1
path=$2
build_dir=$3

docs_dir=docs/elastic
html_dir=$build_dir/html_docs

# Checks if docs clone already exists
if [ ! -d $docs_dir ]; then
    # Only head is cloned
    git clone --depth=1 https://github.com/elastic/docs.git $docs_dir
else
    git -C $docs_dir pull https://github.com/elastic/docs.git
fi

index_list="$(find ${GOPATH%%:*}/src/$path -name 'index.asciidoc')"
for index in $index_list
do
  echo "Building docs for ${name}..."
  echo "Index document: ${index}"
  index_path=$(basename $(dirname $index))
  echo "Index path: $index_path"

  dest_dir="$html_dir/${name}/${index_path}"
  mkdir -p "$dest_dir"
  params="--chunk=1"
  $docs_dir/build_docs --direct_html $params --doc "$index" --out "$dest_dir"
done
