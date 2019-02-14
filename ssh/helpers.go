package ssh

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Host struct {
	User     string
	Port     int
	Hostname string
}

func (h *Host) String() string {
	return fmt.Sprintf("%v@%v:%v", h.User, h.Hostname, h.Port)
}

func sshAgent() (ssh.AuthMethod, error) {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), nil
}

func ConnectToHost(host *Host, timeout time.Duration) (*ssh.Client, error) {
	logger := log.WithFields(log.Fields{
		"hostname": host.Hostname,
		"port":     host.Port,
		"user":     host.User,
	})

	sa, err := sshAgent()
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			sa,
		},
		Timeout:         timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	logger.Debug("Dialing the host")
	sshCon, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.Hostname, host.Port), cfg)
	logger.Debug("Dialed to host successfully")
	if err != nil {
		return nil, errors.Wrapf(err, "could not ssh into server")
	}

	return sshCon, nil
}
