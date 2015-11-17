package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type Instance struct {
	InstanceID, PrivateIP, State           string
	Up                                     time.Duration
	Tags                                   map[string]string
	ICMPPing, SSHPing, HTTPPing, HTTPSPing <-chan PingResponse
}

func NewInstance(i *ec2.Instance) *Instance {
	return &Instance{
		*i.InstanceId, *i.PrivateIpAddress, *i.State.Name,
		time.Since(*i.LaunchTime),
		TagMap(i.Tags),
		ICMPPing(*i.PrivateIpAddress),
		SSHPing(*i.PrivateIpAddress),
		HTTPPing(*i.PrivateIpAddress),
		HTTPSPing(*i.PrivateIpAddress),
	}
}

func TagMap(ts []*ec2.Tag) map[string]string {
	m := map[string]string{}
	for _, t := range ts {
		m[*t.Key] = *t.Value
	}
	return m
}

func (i *Instance) Name() string {
	return i.Tags["Name"]
}

func (i *Instance) String() string {
	return fmt.Sprint(i.InstanceID, " ", i.Name(), " ", i.PrivateIP)
}

func (i *Instance) PrettyState() string {
	var (
		s     = ""
		color = ""
	)
	switch i.State {
	default:
		s = "U"
	case "running":
		s = "R"
		color = "32" // Green
	case "rebooting":
		s = "B"
		color = "34" // Blue
	case "pending":
		s = "P"
		color = "33" // Yellow
	case "stopping":
		s = "-"
		color = "33" // Yellow
	case "shutting-down":
		s = "G"
		color = "33" // Yellow
	case "stopped":
		s = "."
		color = "31" // Red
	case "terminated":
		s = "T"
		color = "31" // Red
	}
	return fmt.Sprint("[" + color + "m" + s + "[m")
}

func InstancesFromEC2Result(in *ec2.DescribeInstancesOutput) []*Instance {
	out := []*Instance{}
	for _, r := range in.Reservations {
		for _, oi := range r.Instances {
			if oi.PrivateIpAddress == nil || oi.PublicIpAddress == nil {
				continue
			}
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
