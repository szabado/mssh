package cli

import (
	"io/ioutil"
	"strings"
	"unicode"

	"github.com/szabado/mssh/ssh"
)

func split(hostList string) []string {
	return strings.FieldsFunc(hostList, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
}

func parseHostsArg(hostsArg string) ([]*ssh.Host, error) {
	hosts := make([]*ssh.Host, 0)
	for _, hostArg := range split(hostsArg) {
		hosts = append(hosts, ssh.ParseHostString(hostArg))
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
