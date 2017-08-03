package model

import "github.com/elastic/beats/libbeat/common"

type FMapping struct {
	Key   string
	Apply func() common.MapStr
}

type SMapping struct {
	Key   string
	Value string
}
