package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "single digit", input: "1", want: "*"},
		{name: "two digits", input: "12", want: "**"},
		{name: "short 3", input: "123", want: "1*3"},
		{name: "short 7", input: "1234567", want: "1*****7"},
		{name: "standard 11", input: "13812345678", want: "138****5678"},
		{name: "with plus", input: "+8613812345678", want: "+86****5678"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, MaskPhone(test.input))
		})
	}
}
