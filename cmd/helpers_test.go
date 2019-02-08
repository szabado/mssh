package cmd

import (
	"os/user"
	"testing"

	a "github.com/stretchr/testify/assert"
	r "github.com/stretchr/testify/require"
)

func TestParseHostsArg(t *testing.T) {
	assert := a.New(t)
	require := r.New(t)

	u, err := user.Current()
	require.Nil(err)
	tests := []struct {
		input  string
		output []*host
		err    bool
	}{
		{
			input: "example.com",
			output: []*host{
				{
					hostName: "example.com",
					port:     defaultPort,
					user:     u.Username,
				},
			},
			err: false,
		},
		{
			input: "user@example.com",
			output: []*host{
				{
					hostName: "example.com",
					port:     defaultPort,
					user:     "user",
				},
			},
			err: false,
		},
		{
			input: "example.com:30",
			output: []*host{
				{
					hostName: "example.com",
					port:     30,
					user:     u.Username,
				},
			},
			err: false,
		},
		{
			input: "user@example.com:30",
			output: []*host{
				{
					hostName: "example.com",
					port:     30,
					user:     "user",
				},
			},
			err: false,
		},
	}

	for i, t := range tests {
		o, err := parseHostsArg(t.input)

		assert.Equalf(t.output, o, "failure in test #%v", i)
		if t.err {
			assert.Errorf(err, "failure in test #%v", i)
		} else {
			assert.NoErrorf(err, "failure in test #%v", i)
		}
	}
}

func TestSplit(t *testing.T) {
	assert := a.New(t)
	//require := r.New(t)

	tests := []struct {
		input  string
		output []string
	}{
		{
			input: "a,b",
			output: []string{
				"a",
				"b",
			},
		},
		{
			input: "a,b",
			output: []string{
				"a",
				"b",
			},
		},
		{
			input: "a, b",
			output: []string{
				"a",
				"b",
			},
		},
		{
			input: "a,b c",
			output: []string{
				"a",
				"b",
				"c",
			},
		},
		{
			input: "a\tb\t\nc",
			output: []string{
				"a",
				"b",
				"c",
			},
		},
	}

	for i, t := range tests {
		s := split(t.input)

		assert.Equalf(t.output, s, "failure in test #%v", i)
	}

}
