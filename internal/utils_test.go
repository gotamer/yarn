package internal

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"git.mills.io/yarnsocial/yarn/types/lextwt"
	"github.com/matryer/is"
	"github.com/stretchr/testify/assert"
)

func TestParseTwtxtUserAgent(t *testing.T) {
	testCases := []struct {
		ua       string
		err      error
		expected *TwtxtUserAgent
	}{
		{
			ua:       `Linguee Bot (http://www.linguee.com/bot; bot@linguee.com)`,
			err:      ErrInvalidUserAgent,
			expected: nil,
		},
		{
			ua:  `twtxt/1.2.3 (+https://foo.com/twtxt.txt; @foo)`,
			err: nil,
			expected: &TwtxtUserAgent{
				Client: "twtxt/1.2.3",
				Nick:   "foo",
				URL:    "https://foo.com/twtxt.txt",
			},
		},
	}

	for _, testCase := range testCases {
		actual, err := ParseTwtxtUserAgent(testCase.ua)
		if err != nil {
			assert.Equal(t, testCase.err, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected.Client, actual.Client)
			assert.Equal(t, testCase.expected.Nick, actual.Nick)
			assert.Equal(t, testCase.expected.URL, actual.URL)
		}
	}
}

func TestFormatMentionsAndTags(t *testing.T) {
	conf := &Config{BaseURL: "http://0.0.0.0:8000"}

	testCases := []struct {
		text     string
		format   TwtTextFormat
		expected string
	}{
		{
			text:     "@<test http://0.0.0.0:8000/user/test/twtxt.txt>",
			format:   HTMLFmt,
			expected: `<a href="http://0.0.0.0:8000/user/test">@test</a>`,
		},
		{
			text:     "@<test http://0.0.0.0:8000/user/test/twtxt.txt>",
			format:   MarkdownFmt,
			expected: "[@test](http://0.0.0.0:8000/user/test/twtxt.txt#test)",
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   HTMLFmt,
			expected: fmt.Sprintf(`<a href="%s">@iamexternal</a>`, URLForExternalProfile(conf, "iamexternal", "http://iamexternal.com/twtxt.txt")),
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   MarkdownFmt,
			expected: "[@iamexternal](http://iamexternal.com/twtxt.txt#iamexternal)",
		},
		{
			text:     "#<test http://0.0.0.0:8000/search?tag=test>",
			format:   HTMLFmt,
			expected: `<a href="http://0.0.0.0:8000/search?tag=test">#test</a>`,
		},
		{
			text:     "#<test http://0.0.0.0:8000/search?tag=test>",
			format:   MarkdownFmt,
			expected: `[#test](http://0.0.0.0:8000/search?tag=test)`,
		},
	}

	for _, testCase := range testCases {
		actual := FormatMentionsAndTags(conf, testCase.text, testCase.format)
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestIsLocalURL(t *testing.T) {
	testCases := []struct {
		url      string
		baseURL  string
		expected bool
	}{
		{
			url:      "https://feeds.twtxt.cc",
			baseURL:  "https://www.twtxt.cc",
			expected: false,
		},
		{
			url:      "http://localhost:8001",
			baseURL:  "http://localhost:8000",
			expected: false,
		},
		{
			url:      "http://localhost:8000/ext",
			baseURL:  "http://localhost:8000",
			expected: true,
		},
	}

	for _, testCase := range testCases {
		actual := strings.HasPrefix(NormalizeURL(testCase.url), NormalizeURL(testCase.baseURL))
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestFormatTwtFactory(t *testing.T) {
	is := is.New(t)

	cfg := NewConfig()
	cfg.baseURL = &url.URL{Host: "example.com"}
	factory := FormatTwtFactory(cfg, NewCache(cfg), &NullArchiver{})
	twter := types.Twter{
		Nick: "test",
		URL:  "https://example.com/twtxt.txt",
	}
	txt := factory(lextwt.NewTwt(twter,
		lextwt.NewDateTime(parseTime("2021-01-24T02:19:54Z"), "2021-01-24T02:19:54Z"),
		lextwt.NewMedia("This is Image", "https://example.com/hot.png", ""),
	), NewUser())

	is.Equal(string(txt), `<p><img title="This is Image" src="//example.com/hot.png" loading="lazy"></p>`+"\n")
}

func parseTime(s string) time.Time {
	if dt, err := time.Parse(time.RFC3339, s); err == nil {
		return dt
	}
	return time.Time{}
}
