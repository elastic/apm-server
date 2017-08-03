// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "github.com/elastic/beats/libbeat/common"

type Config struct {
	Host   string         `config:"host"`
	Server *common.Config `config:"server"`
}

var DefaultConfig = Config{
	Host: "localhost:8080",
}
