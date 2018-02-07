package beater

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/beats/libbeat/logp"
)

func insecureClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

func isServerUp(secure bool, host string, numRetries int, retryInterval time.Duration) bool {
	client := insecureClient()
	var check = func() bool {
		url := url.URL{Scheme: "http", Host: host, Path: "healthcheck"}
		if secure {
			url.Scheme = "https"
		}
		res, err := client.Get(url.String())
		return err == nil && res.StatusCode == 200
	}

	for i := 0; i <= numRetries; i++ {
		if check() {
			logp.NewLogger("http_client").Info("HTTP Server ready")
			return true
		}
		time.Sleep(retryInterval)
	}
	return false
}
