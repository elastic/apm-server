# OS Smoke Tests

This directory contains the smoke tests that are run by CI (`smoke-test-os-sched`). 

## OS Support Tests

OS support tests (`smoke-test-os`) are tests that checks if upcoming APM Server versions work properly in operating systems that [Elastic pledged to support](https://www.elastic.co/support/matrix#matrix_os).
The tests reside in `supported-os/`, and the OS support matrix can be found in `os_matrix.sh`.

The APM Server versions that are tested comes from [active-branches](https://github.com/elastic/oblt-actions/tree/main/elastic/active-branches).
For each version, the test will deploy standalone APM Server to all supported OS on AWS EC2, and check that APM Server ingests events as expected.
