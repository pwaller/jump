package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var publicIP bool

func (i *Instance) preferredIP() string {
	if publicIP {
		return i.PublicIP
	}
	return i.PrivateIP
}

func ShowInstances(instances []*Instance) {
	table := tablewriter.NewWriter(os.Stderr)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetHeader([]string{
		"N", "ID", "Name", "S", "IP Addr", "Launch",
		"ICMP", "SSH", "HTTP", "HTTPS"})

	for n, i := range instances {
		row := []string{
			fmt.Sprint(n), i.InstanceID[2:], i.Name(), i.PrettyState(),
			i.preferredIP(), fmtDuration(i.Up),
			(<-i.ICMPPing).String(),
			(<-i.SSHPing).String(),
			(<-i.HTTPPing).String(),
			(<-i.HTTPSPing).String(),
		}
		table.Append(row)
	}

	table.Render()
}

func GetInstanceFromUser(max int) int {
	s := bufio.NewScanner(os.Stdin)
	if !s.Scan() {
		// User closed stdin before we read anything
		os.Exit(1)
	}
	if s.Err() != nil {
		log.Fatalln("Error reading stdin:", s.Err())
	}
	var n int
	_, err := fmt.Sscan(s.Text(), &n)
	if err != nil {
		log.Fatalln("Unrecognised input:", s.Text())
	}
	if n >= max {
		log.Fatalln("%d is not a valid instance", n)
	}
	return n
}

func InvokeSSH(bastion string, instance *Instance) {
	log.Printf("Connecting: %v", instance.Name())

	args := []string{"/usr/bin/ssh"}

	if bastion != "" {
		format := `ProxyCommand=ssh %v %v %%h %%p`
		// TODO(pwaller): automatically determine available netcat binary?
		netCat := "ncat"
		proxyCommand := fmt.Sprintf(format, bastion, netCat)
		args = append(args, "-o", proxyCommand)
	}

	// Enable the user to specify arguments to the left and right of the host.
	left, right := BreakArgsBySeparator()
	args = append(args, left...)
	args = append(args, instance.preferredIP())
	args = append(args, right...)

	err := syscall.Exec("/usr/bin/ssh", args, os.Environ())
	if err != nil {
		log.Fatalln("Failed to exec:", err)
	}
}

func CursorUp(n int) {
	fmt.Fprint(os.Stderr, "[", n, "F")
}
func ClearToEndOfScreen() {
	fmt.Fprint(os.Stderr, "[", "J")
}

func JumpTo(bastion string, client *ec2.EC2) {

	ec2Instances, err := client.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		log.Fatal("DescribeInstances error:", err)
	}

	// Do this after querying the AWS endpoint (otherwise vulnerable to MITM.)
	ConfigureHTTP(false)

	instances := InstancesFromEC2Result(ec2Instances)
	ShowInstances(instances)

	n := GetInstanceFromUser(len(instances))

	// +1 to account for final newline.
	CursorUp(len(instances) + N_TABLE_DECORATIONS + 1)
	ClearToEndOfScreen()

	InvokeSSH(bastion, instances[n])
}

func Watch(c *ec2.EC2) {

	finish := make(chan struct{})
	go func() {
		defer close(finish)
		// Await stdin closure
		io.Copy(ioutil.Discard, os.Stdin)
	}()

	goUp := func() {}

	for {
		queryStart := time.Now()
		ConfigureHTTP(true)

		ec2Instances, err := c.DescribeInstances(&ec2.DescribeInstancesInput{})
		if err != nil {
			log.Fatal("DescribeInstances error:", err)
		}

		ConfigureHTTP(false)

		instances := InstancesFromEC2Result(ec2Instances)

		goUp()

		ShowInstances(instances)

		queryDuration := time.Since(queryStart)

		select {
		case <-time.After(1*time.Second - queryDuration):
		case <-finish:
			return
		}
		goUp = func() { CursorUp(len(instances) + N_TABLE_DECORATIONS) }
	}

}

const N_TABLE_DECORATIONS = 4

func main() {

	log.SetFlags(0)

	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Fprintln(os.Stderr, "[41;1mWarning: agent forwarding not enabled[K[m")
	}

	if os.Getenv("JUMP_PUBLIC") != "" {
		publicIP = true
	}

	s := session.New()

	if os.Getenv("JUMP_BASTION") != "" {
		// Use the ssh connection to dial remotes
		bastionDialer, err := BastionDialer(os.Getenv("JUMP_BASTION"))
		if err != nil {
			log.Fatalf("BastionDialer: %v", err)
		}
		bastionTransport := &http.Transport{Dial: bastionDialer}

		// The EC2RoleProvider overrides the client configuration if
		// .HTTPClient == http.DefaultClient. Therefore, take a copy.
		// Also, have to re-initialise the default CredChain to make
		// use of HTTPClient set after session.New().
		useClient := *http.DefaultClient
		useClient.Transport = bastionTransport
		s.Config.HTTPClient = &useClient
		s.Config.Credentials = defaults.CredChain(s.Config, defaults.Handlers())

		region, err := ec2metadata.New(s).Region()
		if err != nil {
			log.Printf("Unable to determine bastion region: %v", err)
		}
		// Make API calls from the bastion's region.
		s.Config.Region = aws.String(region)
	}

	client := ec2.New(s)

	if len(os.Args) > 1 && os.Args[1] == "@" {
		Watch(client)
		return
	}

	JumpTo(os.Getenv("JUMP_BASTION"), client)
}
