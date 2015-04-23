package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/awslabs/aws-sdk-go/service/ec2"
)

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

func InvokeSSH(instance *Instance) {
	fmt.Fprintln(os.Stderr, "Connecting to", instance.Name())

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

func CursorUp(n int) {
	fmt.Fprint(os.Stderr, "[", n, "F")
}
func ClearToEndOfScreen() {
	fmt.Fprint(os.Stderr, "[", "J")
}

func JumpTo(client *ec2.EC2) {

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

	InvokeSSH(instances[n])
}

func Watch(client *ec2.EC2) {
	c := ec2.New(nil)

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
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Fprintln(os.Stderr, "[41;1mWarning: agent forwarding not enabled[K[m")
	}

	client := ec2.New(nil)

	if len(os.Args) > 1 && os.Args[1] == "@" {
		Watch(client)
		return
	}

	JumpTo(client)
}
