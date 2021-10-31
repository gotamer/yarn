package types_test

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"git.mills.io/yarnsocial/yarn/types/lextwt"
	"github.com/stretchr/testify/assert"
)

// BenchmarkLextwt-16    	      21	  49342715 ns/op	 6567316 B/op	  178333 allocs/op
func BenchmarkAll(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	wr := nilWriter{}
	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
	opts := mockFmtOpts{"https://twtxt.net"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = f.Seek(0, 0)
				twts, err := parser.fn(f, twter)
				if err != nil {
					fmt.Println(err)
					b.FailNow()
				}
				for _, twt := range twts.Twts() {
					twt.ExpandMentions(opts, opts)
					fmt.Fprintf(wr, "%h", twt)
				}
			}
		})
	}
}

// BenchmarkLextwtParse-16    	      26	  44508742 ns/op	 5450748 B/op	  130290 allocs/op
func BenchmarkParse(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = f.Seek(0, 0)
				_, err := parser.fn(f, twter)
				if err != nil {
					fmt.Println(err)
					b.FailNow()
				}
			}
		})
	}
}

func BenchmarkOutput(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	wr := nilWriter{}
	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
	opts := mockFmtOpts{"https://twtxt.net"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name+"-html", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandMentions(opts, opts)
					twt.FormatText(types.HTMLFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-markdown", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandMentions(opts, opts)
					twt.FormatText(types.MarkdownFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-text", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandMentions(opts, opts)
					twt.FormatText(types.TextFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-literal", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandMentions(opts, opts)
					fmt.Fprintf(wr, "%+l", twt)
				}
			}
		})

	}
}

type nilWriter struct{}

func (nilWriter) Write([]byte) (int, error) { return 0, nil }

type mockFmtOpts struct {
	localURL string
}

func (m mockFmtOpts) LocalURL() *url.URL { u, _ := url.Parse(m.localURL); return u }
func (m mockFmtOpts) IsLocalURL(url string) bool {
	return strings.HasPrefix(url, m.localURL)
}
func (m mockFmtOpts) UserURL(url string) string {
	if strings.HasSuffix(url, "/twtxt.txt") {
		return strings.TrimSuffix(url, "/twtxt.txt")
	}
	return url
}
func (m mockFmtOpts) ExternalURL(nick, uri string) string {
	return fmt.Sprintf(
		"%s/external?uri=%s&nick=%s",
		strings.TrimSuffix(m.localURL, "/"),
		uri, nick,
	)
}
func (m mockFmtOpts) URLForTag(tag string) string {
	return fmt.Sprintf(
		"%s/search?tag=%s",
		strings.TrimSuffix(m.localURL, "/"),
		tag,
	)
}
func (m mockFmtOpts) URLForUser(username string) string {
	return fmt.Sprintf(
		"%s/user/%s/twtxt.txt",
		strings.TrimSuffix(m.localURL, "/"),
		username,
	)
}
func (m mockFmtOpts) FeedLookup(s string) *types.Twter {
	return &types.Twter{Nick: s, URL: fmt.Sprintf("https://example.com/users/%s/twtxt.txt", s)}
}

type preambleTestCase struct {
	in       string
	preamble string
	drain    string
}

func TestSplitTwts(t *testing.T) {
	assert := assert.New(t)

	twter := types.Twter{}

	t.Run("FutureNewOld", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now().Add(time.Minute), "1m"),
			types.MakeTwt(twter, time.Now(), "0s"),
			types.MakeTwt(twter, time.Now().Add(-time.Minute), "-1m"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 1)
		assert.Equal(1, len(f))
		assert.Equal(f[0], twts[0])
		assert.Equal(1, len(n))
		assert.Equal(n[0], twts[1])
		assert.Equal(1, len(o))
		assert.Equal(o[0], twts[2])
	})

	t.Run("AllFuture", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now().Add(5*time.Minute), "5m"),
			types.MakeTwt(twter, time.Now().Add(3*time.Minute), "3m"),
			types.MakeTwt(twter, time.Now().Add(time.Minute), "1m"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 1)
		assert.Equal(3, len(f))
		assert.Equal(f[0], twts[0])
		assert.Equal(f[1], twts[1])
		assert.Equal(f[2], twts[2])
		assert.Equal(0, len(n))
		assert.Equal(0, len(o))
	})

	t.Run("AllOld", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now().Add(-5*time.Minute), "-5m"),
			types.MakeTwt(twter, time.Now().Add(-3*time.Minute), "-3m"),
			types.MakeTwt(twter, time.Now().Add(-time.Minute), "-1m"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 1)
		assert.Equal(0, len(f))
		assert.Equal(0, len(n))
		assert.Equal(3, len(o))
		assert.Equal(o[0], twts[0])
		assert.Equal(o[1], twts[1])
		assert.Equal(o[2], twts[2])
	})

	t.Run("AllNew", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now(), "0s"),
			types.MakeTwt(twter, time.Now().Add(-5*time.Second), "-5s"),
			types.MakeTwt(twter, time.Now().Add(-3*time.Second), "-3s"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 3)
		assert.Equal(0, len(f))
		assert.Equal(3, len(n))
		assert.Equal(n[0], twts[0])
		assert.Equal(n[1], twts[1])
		assert.Equal(n[2], twts[2])
		assert.Equal(0, len(o))
	})

	t.Run("AllNewLimited", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now(), "0s"),
			types.MakeTwt(twter, time.Now().Add(-5*time.Second), "-5s"),
			types.MakeTwt(twter, time.Now().Add(-4*time.Second), "-4s"),
			types.MakeTwt(twter, time.Now().Add(-3*time.Second), "-3s"),
			types.MakeTwt(twter, time.Now().Add(-2*time.Second), "-2s"),
			types.MakeTwt(twter, time.Now().Add(-1*time.Second), "-1s"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 3)
		assert.Equal(0, len(f))
		assert.Equal(3, len(n))
		assert.Equal(n[0], twts[0])
		assert.Equal(n[1], twts[1])
		assert.Equal(n[2], twts[2])
		assert.Equal(3, len(o))
		assert.Equal(o[0], twts[3])
		assert.Equal(o[1], twts[4])
		assert.Equal(o[2], twts[5])
	})

	t.Run("Mixture", func(t *testing.T) {
		twts := types.Twts{
			types.MakeTwt(twter, time.Now().Add(5*time.Minute), "5m"),
			types.MakeTwt(twter, time.Now().Add(3*time.Minute), "3m"),
			types.MakeTwt(twter, time.Now().Add(time.Minute), "1m"),
			types.MakeTwt(twter, time.Now(), "0s"),
			types.MakeTwt(twter, time.Now().Add(-5*time.Second), "-5s"),
			types.MakeTwt(twter, time.Now().Add(-4*time.Second), "-4s"),
			types.MakeTwt(twter, time.Now().Add(-3*time.Second), "-3s"),
			types.MakeTwt(twter, time.Now().Add(-2*time.Second), "-2s"),
			types.MakeTwt(twter, time.Now().Add(-1*time.Second), "-1s"),
		}

		f, n, o := types.SplitTwts(twts, time.Minute, 3)
		assert.Equal(3, len(f))
		assert.Equal(f[0], twts[0])
		assert.Equal(f[1], twts[1])
		assert.Equal(f[2], twts[2])
		assert.Equal(3, len(n))
		assert.Equal(n[0], twts[3])
		assert.Equal(n[1], twts[4])
		assert.Equal(n[2], twts[5])
		assert.Equal(3, len(o))
		assert.Equal(o[0], twts[6])
		assert.Equal(o[1], twts[7])
		assert.Equal(o[2], twts[8])
	})
}
