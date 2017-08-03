package tests

import (
	"strings"

	"github.com/elastic/beats/libbeat/common"
)

func FlattenCommonMapStr(m common.MapStr, prefix string, keysBlacklist []string, flattened []string) []string {
	for k, v := range m {
		flattenedKey := StrConcat(prefix, k, ".")
		if !isBlacklistedKey(keysBlacklist, flattenedKey) {
			flattened = append(flattened, flattenedKey)
		}
		if vMapStr, ok := v.(common.MapStr); ok {
			flattened = FlattenCommonMapStr(vMapStr, flattenedKey, keysBlacklist, flattened)
		}
	}
	return flattened
}

func isBlacklistedKey(keysBlacklist []string, key string) bool {
	for _, disabledKey := range keysBlacklist {
		if strings.HasPrefix(key, disabledKey) {
			return true
		}
	}
	return false
}
