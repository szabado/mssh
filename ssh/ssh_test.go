package ssh

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestParseHostsArg(t *testing.T) {
	require := r.New(t)

	tests := []struct {
		input  string
		output *Host
	}{
		{
			input: "example.com",
			output: &Host{
				Hostname: "example.com",
				Port:     defaultPort,
				User:     defaultUser,
			},
		},
		{
			input: "user@example.com",
			output: &Host{
				Hostname: "example.com",
				Port:     defaultPort,
				User:     "user",
			},
		},
		{
			input: "example.com:30",
			output: &Host{
				Hostname: "example.com",
				Port:     30,
				User:     defaultUser,
			},
		},
		{
			input: "user@example.com:30",
			output: &Host{
				Hostname: "example.com",
				Port:     30,
				User:     "user",
			},
		},
	}

	for _, t := range tests {
		o := ParseHostString(t.input)

		require.Equal(t.output, o)
	}
}
