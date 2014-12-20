package main

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"time"
)

type PingResponse struct {
	OK       bool
	Duration time.Duration
}

func (p PingResponse) String() string {
	var (
		result = "#"
		color  = "31" // Red
	)

	if p.OK {
		result = fmt.Sprintf("%.0f", p.Duration.Seconds()*1000)
		if p.Duration < 100*time.Millisecond {
			color = "33" // Yellow
		}
		if p.Duration < 50*time.Millisecond {
			color = "32" // Green
		}
	}

	return "[" + color + "m" + result + "[m"
}

// Measure the time required to execute f().
// f() should take care not to block forever.
func DoPing(f func() error) <-chan PingResponse {
	respChan := make(chan PingResponse)
	go func() {
		start := time.Now()
		err := f()
		responseTime := time.Since(start)
		respChan <- PingResponse{err == nil, responseTime}
	}()
	return respChan
}

func SSHPing(where string) <-chan PingResponse {
	return DoPing(func() error {
		d := &net.Dialer{Timeout: time.Second}
		c, err := d.Dial("tcp", where+":22")
		if err == nil {
			c.Close()
		}
		return err
	})
}

func HTTPPing(where string) <-chan PingResponse {
	return DoPing(func() error {
		_, err := http.Get(fmt.Sprint("http://", where))
		return err
	})
}

func HTTPSPing(where string) <-chan PingResponse {
	return DoPing(func() error {
		_, err := http.Get(fmt.Sprint("https://", where))
		return err
	})
}

func ICMPPing(where string) <-chan PingResponse {
	// -c: Count, -W timeout in seconds
	return DoPing(exec.Command("ping", "-W1", "-c1", where).Run)
}
