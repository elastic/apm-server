package server

import (
	"time"
)

type Config struct {
	MaxUnzippedSize int64         `config:"max_unzipped_size"`
	MaxHeaderBytes  int           `config:"max_header_bytes"`
	ReadTimeout     time.Duration `config:"read_timeout"`
	WriteTimeout    time.Duration `config:"write_timeout"`
	SecretToken     string        `config:"secret_token"`
	SSLEnabled      bool          `config:"ssl.enabled"`
	SSLPrivateKey   string        `config:"ssl.key"`
	SSLCert         string        `config:"ssl.certificate"`
}

var defaultConfig = Config{
	MaxUnzippedSize: 10 * 1024 * 1024, // 10mb
	MaxHeaderBytes:  1048576,          // 1mb
	ReadTimeout:     2 * time.Second,
	WriteTimeout:    2 * time.Second,
	SecretToken:     "",
}
