package main

import (
	"crypto/tls"
	"net/http"
	"os"
	"time"
)

func BreakArgsBySeparator() (left, right []string) {
	seenSeparator := false
	for _, param := range os.Args[1:] {
		if param == "--" {
			seenSeparator = true
			continue
		}
		if seenSeparator {
			right = append(right, param)
		} else {
			left = append(left, param)
		}
	}
	return
}

// Configure HTTP for 1s timeout and HTTPS to ignore SSL CA errors.
// if `externalRequest` is true, sets it back to the go defaults, otherwise
// ensures that CA violations are treated as errors
func ConfigureHTTP(externalRequest bool) {
	if externalRequest {
		http.DefaultClient.Timeout = 0
		t := http.DefaultTransport.(*http.Transport)
		// Use the default TLS config
		t.TLSClientConfig = nil
		return
	}

	http.DefaultClient.Timeout = 1 * time.Second

	t := http.DefaultTransport.(*http.Transport)
	// Ignore SSL certificate errors
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
}
