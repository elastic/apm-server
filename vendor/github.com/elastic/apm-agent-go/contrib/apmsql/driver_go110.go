// +build go1.10

package apmsql

import (
	"context"
	"database/sql/driver"

	"github.com/elastic/apm-agent-go"
)

func (d *tracingDriver) OpenConnector(name string) (driver.Connector, error) {
	if dc, ok := d.Driver.(driver.DriverContext); ok {
		oc, err := dc.OpenConnector(name)
		if err != nil {
			return nil, err
		}
		return &driverConnector{oc.Connect, d, name}, nil
	}
	connect := func(context.Context) (driver.Conn, error) {
		return d.Driver.Open(name)
	}
	return &driverConnector{connect, d, name}, nil
}

type driverConnector struct {
	connect func(context.Context) (driver.Conn, error)
	driver  *tracingDriver
	name    string
}

func (d *driverConnector) Connect(ctx context.Context) (driver.Conn, error) {
	span, ctx := elasticapm.StartSpan(ctx, "connect", d.driver.connectSpanType)
	if span != nil {
		defer span.Done(-1)
	}
	conn, err := d.connect(ctx)
	if err != nil {
		return nil, err
	}
	return newConn(conn, d.driver, d.name), nil
}

func (d *driverConnector) Driver() driver.Driver {
	return d.driver
}
