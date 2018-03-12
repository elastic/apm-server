package utility

import (
	"net/url"
	"path"
)

func CleanUrlPath(p string) string {
	url, err := url.Parse(p)
	if err != nil {
		return p
	}
	url.Path = path.Clean(url.Path)
	return url.String()
}
