## Rally worker module

This modules sets up [rally daemons](https://esrally.readthedocs.io/en/stable/rally_daemon.html) which allows for distributed load generation. Such setups can generate high throughput loads and are preferred for testing large ES clusters. The module also runs `rally` with the configured arguments each time `terraform apply` is executed.
