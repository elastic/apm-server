[![Functional Tests](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml/badge.svg)](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml)

# Functional Tests

The functional tests test that APM Server works as expected after version upgrades.

## Running the Tests

To run the tests, you will first need to set the `EC_API_KEY` environment variable.

Then, from the current directory, simply run:
```sh
go test -v -timeout=30m -cleanup-on-failure=false -target "qa" ./
```

## Structure of the Tests

We suggest each upgrade test to be named in the format of `TestUpgrade_<from_version>_to_<to_version_1>[_to_<to_version_N>]*[_<suffix>]?`.
This means that the test will start from `from_version`, and be upgraded to `to_version_1`, then subsequently to
`to_version_2` etc. all the way to `to_version_N`.

The file that each test is in is named after the last minor version in the upgrade chain. For example, if the test name
is `TestUpgrade_7_17_0_to_8_18_0_to_9_0_0_Something`, it should be written in `9_0_test.go`.

The Terraforms for each test is located at the directory with the same name as the test.
At this moment the terraform code must be copied in a separate folder for each test. We plan to address this duplication later on.