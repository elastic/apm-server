# apmsql

Package apmsql provides a wrapper for database/sql/driver.Drivers
for tracing database operations as spans of a transaction traced
by Elastic APM.

To instrument a driver, you can simply swap your application's
calls to [sql.Register](https://golang.org/pkg/database/sql/#Register)
and [sql.Open](https://golang.org/pkg/database/sql/#Open) to
apmsql.Register and apmsql.Open respectively. The apmsql.Register
function accepts zero or more options to influence how tracing
is performed.
