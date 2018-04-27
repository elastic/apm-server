package apmsql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"

	"github.com/elastic/apm-agent-go/contrib/apmsql/dsn"
)

// DriverPrefix should be used as a driver name prefix when
// registering via sql.Register.
const DriverPrefix = "elasticapm/"

// Register registers a traced version of the given driver.
//
// The name and driver values should be the same as given to
// sql.Register: the name of the driver (e.g. "postgres"), and
// the driver (e.g. &github.com/lib/pq.Driver{}).
func Register(name string, driver driver.Driver, opts ...WrapOption) {
	wrapped := Wrap(driver, opts...)
	sql.Register(DriverPrefix+name, wrapped)
}

// Open opens a database with the given driver and data source names,
// as in sql.Open. The driver name should be one registered via the
// Register function in this package.
func Open(driverName, dataSourceName string) (*sql.DB, error) {
	return sql.Open(DriverPrefix+driverName, dataSourceName)
}

// Wrap wraps a database/sql/driver.Driver such that
// the driver's database methods are traced. The tracer
// will be obtained from the context supplied to methods
// that accept it.
func Wrap(driver driver.Driver, opts ...WrapOption) driver.Driver {
	d := &tracingDriver{
		Driver: driver,
	}
	for _, opt := range opts {
		opt(d)
	}
	if d.driverName == "" {
		d.driverName = driverName(driver)
	}
	if d.dsnParser == nil {
		d.dsnParser = dsnParser(d.driverName)
	}
	// store span types to avoid repeat allocations
	d.connectSpanType = d.formatSpanType("connect")
	d.pingSpanType = d.formatSpanType("ping")
	d.prepareSpanType = d.formatSpanType("prepare")
	d.querySpanType = d.formatSpanType("query")
	d.execSpanType = d.formatSpanType("exec")
	return d
}

// WrapOption is an option that can be supplied to Wrap.
type WrapOption func(*tracingDriver)

// WithDriverName returns a WrapOption which sets the underlying
// driver name to the specified value. If WithDriverName is not
// supplied to Wrap, the driver name will be inferred from the
// driver supplied to Wrap.
func WithDriverName(name string) WrapOption {
	return func(d *tracingDriver) {
		d.driverName = name
	}
}

// WithDSNParser returns a WrapOption which sets the function to
// use for parsing the data source name. If WithDSNParser is not
// supplied to Wrap, the function to use will be inferred from
// the driver name.
func WithDSNParser(f dsn.ParserFunc) WrapOption {
	return func(d *tracingDriver) {
		d.dsnParser = f
	}
}

type tracingDriver struct {
	driver.Driver
	driverName string
	dsnParser  dsn.ParserFunc

	connectSpanType string
	execSpanType    string
	pingSpanType    string
	prepareSpanType string
	querySpanType   string
}

func (d *tracingDriver) formatSpanType(suffix string) string {
	return fmt.Sprintf("db.%s.%s", d.driverName, suffix)
}

// querySignature returns the value to use in Span.Name for
// a database query.
func (d *tracingDriver) querySignature(query string) string {
	// TODO(axw) parse statement. Create a WrapOption for
	// consumers to provide a driver-specific parser if
	// necessary.
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func (d *tracingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return newConn(conn, d, name), nil
}

func driverName(d driver.Driver) string {
	t := reflect.TypeOf(d)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Name() {
	case "SQLiteDriver":
		return "sqlite3"
	case "MySQLDriver":
		return "mysql"
	case "Driver":
		// Check suffix in case of vendoring.
		if strings.HasSuffix(t.PkgPath(), "github.com/lib/pq") {
			return "postgresql"
		}
	}
	// TODO include the package path of the driver in context
	// so we can easily determine how the rules above should
	// be updated.
	return "generic"
}
