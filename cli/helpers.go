package cli

import (
	"io/ioutil"
	"os/user"
	"strconv"
	"strings"
	"unicode"

	"github.com/szabado/mssh/ssh"
)

const (
	defaultPort = 22
)

func split(hostList string) []string {
	return strings.FieldsFunc(hostList, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
}

func parseHostsArg(hostsArg string) ([]*ssh.Host, error) {
	hosts := make([]*ssh.Host, 0)
	username := ""
	if u, err := user.Current(); err != nil {
		return hosts, err
	} else {
		username = u.Username
	}

	for _, hostArg := range split(hostsArg) {
		h := &ssh.Host{
			User:     username,
			Hostname: hostArg,
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

		hosts = append(hosts, h)
	}
	return hosts, nil
}

func loadFileContents(file string) (string, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
