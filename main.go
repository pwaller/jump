package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/stripe/aws-go/aws"
	"github.com/stripe/aws-go/gen/ec2"
)

var AWS_REGION = os.Getenv("AWS_REGION")

func init() {
	// Default the region to eu-west-1.
	if AWS_REGION == "" {
		AWS_REGION = "eu-west-1"
	}
}

type PingResponse struct {
	OK       bool
	Duration time.Duration
}

func (p PingResponse) String() string {
	var (
		result = "-"
		color  = "31"
	)

	if p.OK {
		result = fmt.Sprintf("%.01fms", p.Duration.Seconds()*1000)
		if p.Duration < 100*time.Millisecond {
			color = "32"
		}
	}

	return "\033[" + color + "m" + result + "\033[m"
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

func Ping(where string) <-chan PingResponse {
	// -c: Count, -W timeout in seconds
	return DoPing(exec.Command("ping", "-W1", "-c1", where).Run)
}

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

type Instance struct {
	InstanceID, PrivateIP              string
	Tags                               map[string]string
	Ping, SSHPing, HTTPPing, HTTPSPing <-chan PingResponse
}

func TagMap(ts []ec2.Tag) map[string]string {
	m := map[string]string{}
	for _, t := range ts {
		m[*t.Key] = *t.Value
	}
	return m
}

func NewInstance(i ec2.Instance) *Instance {
	ping := Ping(*i.PrivateIPAddress)
	sshPing := SSHPing(*i.PrivateIPAddress)
	httpPing := HTTPPing(*i.PrivateIPAddress)
	httpsPing := HTTPSPing(*i.PrivateIPAddress)
	return &Instance{
		*i.InstanceID, *i.PrivateIPAddress, TagMap(i.Tags),
		ping, sshPing, httpPing, httpsPing}
}

func (i *Instance) Name() string {
	return i.Tags["Name"]
}

func (i *Instance) String() string {
	return fmt.Sprint(i.InstanceID, " ", i.Name(), " ", i.PrivateIP)
}

func InstancesFromEC2Result(in *ec2.DescribeInstancesResult) []*Instance {
	out := []*Instance{}
	for _, r := range in.Reservations {
		for _, oi := range r.Instances {
			out = append(out, NewInstance(oi))
		}
	}
	sort.Sort(InstancesByName(out))
	return out
}

type InstancesByName []*Instance

func (a InstancesByName) Len() int           { return len(a) }
func (a InstancesByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a InstancesByName) Less(i, j int) bool { return a[i].Name() < a[j].Name() }

// Configure HTTP for 1s timeout and HTTPS to ignore SSL CA errors.
func ConfigureHTTP() {

	http.DefaultClient.Timeout = 1 * time.Second

	t := http.DefaultTransport.(*http.Transport)

	// Ignore SSL certificate errors
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
}

func main() {

	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Fprintln(os.Stderr, "[41;1mWarning: agent forwarding not enabled[K[m")
	}

	creds := aws.IAMCreds()

	c := ec2.New(creds, AWS_REGION, nil)

	resp, err := c.DescribeInstances(&ec2.DescribeInstancesRequest{})
	if err != nil {
		log.Fatal("DescribeInstances error:", err)
	}

	// Do this after querying the AWS endpoint (otherwise vulnerable to MITM.)
	ConfigureHTTP()

	instances := InstancesFromEC2Result(resp)

	table := tablewriter.NewWriter(os.Stderr)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetHeader([]string{"N", "InstanceID", "Name", "PrivateIP",
		"ICMP", "SSH", "HTTP", "HTTPS"})

	for n, i := range instances {
		row := []string{
			fmt.Sprint(n), i.InstanceID, i.Name(), i.PrivateIP,
			(<-i.Ping).String(),
			(<-i.SSHPing).String(),
			(<-i.HTTPPing).String(),
			(<-i.HTTPSPing).String(),
		}
		table.Append(row)
	}

	table.Render()

	s := bufio.NewScanner(os.Stdin)
	if s.Err() != nil || !s.Scan() {
		log.Fatalln("Error reading stdin:", s.Err())
	}

	var n int
	_, err = fmt.Sscan(s.Text(), &n)
	if err != nil {
		log.Fatalln("Unrecognised input:", s.Text())
	}
	if n >= len(instances) {
		log.Fatalln("%d is not a valid instance", n)
	}

	instance := instances[n]
	log.Println("Sending you to:", instance)

	args := []string{"/usr/bin/ssh"}
	args = append(args, os.Args[1:]...)
	args = append(args, instance.PrivateIP)

	err = syscall.Exec("/usr/bin/ssh", args, os.Environ())
	if err != nil {
		log.Fatalln("Failed to exec:", err)
	}
}
