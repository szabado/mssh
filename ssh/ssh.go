package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	defaultPort = -1
	defaultUser = ""
)

type Host struct {
	User     string
	Port     int
	Hostname string
}

func (h *Host) String() string {
	s := h.Hostname
	if h.User != defaultUser {
		s = fmt.Sprintf("%s@%s", h.User, s)
	}

	if h.Port != defaultPort {
		s = fmt.Sprintf("%s:%v", s, h.Port)
	}

	return s
}

func ParseHostString(s string) *Host {
	h := &Host{
		User:     defaultUser,
		Hostname: s,
		Port:     defaultPort,
	}

	if strings.Contains(h.Hostname, "@") {
		parts := strings.Split(h.Hostname, "@")
		h.User = parts[0]
		h.Hostname = parts[1]
	}

	if strings.Contains(h.Hostname, ":") {
		parts := strings.Split(h.Hostname, ":")
		if p, err := strconv.ParseInt(parts[1], 10, 32); err == nil {
			h.Hostname = parts[0]
			h.Port = int(p)
		}
	}
	return h
}

func sshAgent() (ssh.AuthMethod, error) {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), nil
}

func ConnectToHost(host *Host, timeout time.Duration) (*ssh.Client, error) {
	logger := log.WithField("host", fmt.Sprintf("%s", host))

	sa, err := sshAgent()
	if err != nil {
		return nil, err
	}

	if host.User == defaultUser {
		if u, err := user.Current(); err != nil {
			return nil, err
		} else {
			host.User = u.Username
		}
	}

	if host.Port == defaultPort {
		host.Port = 22
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

func RunCommand(h *Host, command string, timeout time.Duration) ([]byte, error) {
	logger := log.WithFields(log.Fields{
		"hostname": h.Hostname,
		"port":     h.Port,
		"user":     h.User,
	})

	c, err := ConnectToHost(h, timeout)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	logger.Debug("Establishing new session")
	s, err := c.NewSession()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	logger.WithField("command", command).Debug("Running command")
	o, err := s.CombinedOutput(command)
	logger.WithField("command", command).Debug("Command finished")

	return o, err
}

func RunCommandWithOpenSSH(h *Host, command string) ([]byte, error) {
	c := exec.Command("ssh", h.String(), command)
	return c.CombinedOutput()
}
