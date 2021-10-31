package types_test

import (
	std_ioutil "io/ioutil"
	"strings"
	"testing"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/badgerodon/ioutil"
	"github.com/stretchr/testify/assert"
)

func TestPreambleFeed(t *testing.T) {
	assert := assert.New(t)

	tests := []preambleTestCase{
		{
			in:       "# testing\n\n2020-...",
			preamble: "# testing",
			drain:    "\n\n2020-...",
		},

		{
			in:       "# testing\nmulti\nlines\n\n2020-...",
			preamble: "# testing\nmulti\nlines",
			drain:    "\n\n2020-...",
		},

		{
			in:       "2020-...NO PREAMBLE",
			preamble: "",
			drain:    "2020-...NO PREAMBLE",
		},

		{
			in:       "#onlyonen\n2020-...OOPS ALL PREAMBLE",
			preamble: "#onlyonen\n2020-...OOPS ALL PREAMBLE",
			drain:    "",
		},

		{
			in:       "#onlypreamble\n",
			preamble: "#onlypreamble\n",
			drain:    "",
		},

		{
			in:       "",
			preamble: "",
			drain:    "",
		},

		{
			in:       "X",
			preamble: "",
			drain:    "X",
		},

		{
			in:       "#",
			preamble: "#",
			drain:    "",
		},
	}

	for _, tt := range tests {
		t.Run("Read", func(t *testing.T) {
			pf, err := types.ReadPreambleFeed(strings.NewReader(tt.in), int64(len(tt.in)))
			assert.NoError(err)
			assert.Equal(tt.preamble, pf.Preamble())

			drain, err := std_ioutil.ReadAll(pf)
			assert.NoError(err)
			assert.Equal(tt.drain, string(drain))
		})

		t.Run("Stream", func(t *testing.T) {
			pf, err := types.ReadPreambleFeed(strings.NewReader(tt.in), int64(len(tt.in)))
			assert.NoError(err)
			assert.Equal(tt.preamble, pf.Preamble())

			mrs := ioutil.NewMultiReadSeeker(strings.NewReader(pf.Preamble()), pf)
			data, err := std_ioutil.ReadAll(mrs)
			assert.NoError(err)
			assert.Equal(tt.in, string(data))
		})
	}
}
