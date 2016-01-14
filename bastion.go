package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/user"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// BastionDialer returns a bastion
func BastionDialer(
	bastionHost string,
) (func(network, addr string) (net.Conn, error), error) {
	auths := []ssh.AuthMethod{}
	auths = append(auths, agentAuth()...)

	config := &ssh.ClientConfig{}
	config.SetDefaults()

	u, err := url.Parse("//" + bastionHost)
	if err != nil {
		return nil, err
	}

	if u.User == nil || u.User.Username() == "" {
		whoami, err := user.Current()
		if err != nil {
			return nil, err
		}
		u.User = url.User(whoami.Username)
	}

	_, _, err = net.SplitHostPort(u.Host)
	if err != nil {
		u.Host += ":ssh"
	}

	config.User = u.User.Username()
	config.Auth = auths

	log.Printf("Using bastion host: %v", u.Host)
	conn, err := ssh.Dial("tcp", u.Host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %q: %v", bastionHost, err)
	}
	return conn.Dial, nil
}

func agentAuth() (auths []ssh.AuthMethod) {
	if sock := os.Getenv("SSH_AUTH_SOCK"); len(sock) > 0 {
		if agconn, err := net.Dial("unix", sock); err == nil {
			ag := agent.NewClient(agconn)
			auths = append(auths, ssh.PublicKeysCallback(ag.Signers))
		}
	}
	return auths
}
