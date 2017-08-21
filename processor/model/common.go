package model

import "github.com/elastic/beats/libbeat/common"

type DocMapping struct {
	Key   string
	Apply func() common.MapStr
}
