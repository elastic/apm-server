package beater

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"

	"github.com/elastic/go-ucfg"
)

func init() {
	if err := ucfg.RegisterValidator("maxlen", func(v interface{}, param string) error {
		if v == nil {
			return nil
		}
		switch v := reflect.ValueOf(v); v.Kind() {
		case reflect.Array, reflect.Map, reflect.Slice:
			maxlen, err := strconv.ParseInt(param, 0, 64)
			if err != nil {
				return err
			}

			if length := int64(v.Len()); length > maxlen {
				return fmt.Errorf("requires length (%d) <= %v", length, param)
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

type urls []*url.URL

func (u *urls) Unpack(c interface{}) error {
	if c == nil {
		return nil
	}
	hosts, ok := c.([]interface{})
	if !ok {
		return fmt.Errorf("hosts must be a list, got: %#v", c)
	}

	nu := make(urls, len(hosts))
	for i, host := range hosts {
		h, ok := host.(string)
		if !ok {
			return fmt.Errorf("host must be a string, got: %#v", h)
		}
		url, err := url.Parse(h)
		if err != nil {
			return err
		}
		nu[i] = url
	}
	*u = nu

	return nil
}
