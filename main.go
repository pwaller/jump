package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"syscall"

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

func ShowInstances(instances []*Instance) {
	table := tablewriter.NewWriter(os.Stderr)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetHeader([]string{
		"N", "ID", "Name", "S", "Private", "Launch",
		"ICMP", "SSH", "HTTP", "HTTPS"})

	for n, i := range instances {
		row := []string{
			fmt.Sprint(n), i.InstanceID[2:], i.Name(), i.PrettyState(),
			i.PrivateIP, fmtDuration(i.Up),
			(<-i.ICMPPing).String(),
			(<-i.SSHPing).String(),
			(<-i.HTTPPing).String(),
			(<-i.HTTPSPing).String(),
		}
		table.Append(row)
	}

	table.Render()
}

func UserSelectInstance(max int) int {
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

func InvokeSSH(instance *Instance) {
	log.Println("Sending you to:", instance)

	args := []string{"/usr/bin/ssh"}
	// Enable the user to specify arguments to the left and right of the host.
	left, right := BreakArgsBySeparator()
	args = append(args, left...)
	args = append(args, instance.PrivateIP)
	args = append(args, right...)

	err := syscall.Exec("/usr/bin/ssh", args, os.Environ())
	if err != nil {
		log.Fatalln("Failed to exec:", err)
	}
}

func main() {

	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Fprintln(os.Stderr, "[41;1mWarning: agent forwarding not enabled[K[m")
	}

	creds := aws.IAMCreds()
	c := ec2.New(creds, AWS_REGION, nil)
	ec2Instances, err := c.DescribeInstances(&ec2.DescribeInstancesRequest{})
	if err != nil {
		log.Fatal("DescribeInstances error:", err)
	}

	// Do this after querying the AWS endpoint (otherwise vulnerable to MITM.)
	ConfigureHTTP()

	instances := InstancesFromEC2Result(ec2Instances)
	ShowInstances(instances)

	n := UserSelectInstance(len(instances))
	InvokeSSH(instances[n])
}
