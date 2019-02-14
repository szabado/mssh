package cli

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	require := r.New(t)

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

	for _, t := range tests {
		s := split(t.input)

		require.Equal(t.output, s)
	}
}
