[![ci](https://github.com/elastic/apm-data/actions/workflows/ci.yml/badge.svg)](https://github.com/elastic/apm-data/actions/workflows/ci.yml)

# apm-data

apm-data holds definitions and code for manipulating Elastic APM data.

### Code Generation

Whenever updating protobuf files, you need to manually run the generation of
the related code, as well as licenses update.

```
make all
```

## License

This software is licensed under the [Apache 2 license](https://github.com/elastic/apm-data/blob/main/LICENSE).
