package lextwt

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	log "github.com/sirupsen/logrus"
)

func DefaultTwtManager() {
	types.SetTwtManager(&lextwtManager{})
}

// ParseFile and return time & count limited twts + comments
func ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {

	f := &lextwtFile{twter: &twter}

	nTwts, nErrors := 0, 0

	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(&twter)

	for !parser.IsEOF() {
		line := parser.ParseLine()

		switch e := line.(type) {
		case *Comment:
			f.comments = append(f.comments, e)
		case *Twt:
			if e.IsNil() {
				log.Errorf("invalid feed or bad line parsing %#v", twter)
				nErrors++
				continue
			}

			nTwts++
			f.twts = append(f.twts, e)

			// If the twt has an override twter add to authors.
			if e.twter.URL != f.twter.URL {
				found := false
				for i := range f.twters {
					if f.twters[i].URL == e.twter.URL {
						found = true
						// de-dup the elements twter with the file one.
						e.twter = f.twters[i]
					}
				}
				// only add to authors if not seen before.
				if !found {
					f.twters = append(f.twters, e.twter)
				}
			}
		}
	}
	nErrors += len(parser.Errs())

	if nTwts == 0 && nErrors > 0 {
		log.Warnf("erroneous feed dtected (%d twts parsed %d errors)", nTwts, nErrors)
		return nil, types.ErrInvalidFeed
	}

	if v, ok := f.Info().GetN("nick", 0); ok {
		if strings.TrimSpace(v.Value()) != "" {
			f.twter.Nick = v.Value()
		}
	}

	if v, ok := f.Info().GetN("url", 0); ok {
		if strings.TrimSpace(v.Value()) != "" {
			if _, err := url.Parse(v.Value()); err == nil {
				f.twter.URL = v.Value()
			}
		}
	}

	if v, ok := f.Info().GetN("twturl", 0); ok {
		if strings.TrimSpace(v.Value()) != "" {
			if _, err := url.Parse(v.Value()); err == nil {
				f.twter.URL = v.Value()
			}
		}
	}

	if v, ok := f.Info().GetN("avatar", 0); ok {
		if strings.TrimSpace(v.Value()) != "" {
			f.twter.Avatar = v.Value()
		}
	}

	if v, ok := f.Info().GetN("description", 0); ok {
		if strings.TrimSpace(v.Value()) != "" {
			f.twter.Tagline = v.Value()
		}
	}

	if v, ok := f.Info().GetN("following", 0); ok {
		if n, err := strconv.Atoi(v.Value()); err == nil {
			f.twter.Following = n
		}
	} else {
		f.twter.Following = len(f.Info().Followers())
	}

	if v, ok := f.Info().GetN("followers", 0); ok {
		if n, err := strconv.Atoi(v.Value()); err == nil {
			f.twter.Followers = n
		}
	}

	return f, nil
}
func ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	if line == "" {
		return types.NilTwt, nil
	}

	r := strings.NewReader(line)
	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(&twter)

	twt = parser.ParseTwt()

	if twt.IsZero() {
		return types.NilTwt, fmt.Errorf("Empty Twt: %s", line)
	}

	return twt, err
}

type lextwtManager struct{}

func (*lextwtManager) DecodeJSON(b []byte) (types.Twt, error) { return DecodeJSON(b) }
func (*lextwtManager) ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	return ParseLine(line, twter)
}
func (*lextwtManager) ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {
	return ParseFile(r, twter)
}
func (*lextwtManager) MakeTwt(twter types.Twter, ts time.Time, text string) types.Twt {
	dt := NewDateTime(ts, "")
	elems, err := ParseText(text)
	if err != nil {
		return types.NilTwt
	}

	twt := NewTwt(twter, dt, elems...)
	if err != nil {
		return types.NilTwt
	}

	return twt
}

type lextwtFile struct {
	twter    *types.Twter
	twters   []*types.Twter
	twts     types.Twts
	comments Comments
}

var _ types.TwtFile = (*lextwtFile)(nil)

func NewTwtFile(twter types.Twter, comments Comments, twts types.Twts) *lextwtFile {
	return &lextwtFile{&twter, []*types.Twter{&twter}, twts, comments}
}
func (r *lextwtFile) Twter() types.Twter      { return *r.twter }
func (r *lextwtFile) Authors() []*types.Twter { return r.twters }
func (r *lextwtFile) Info() types.Info        { return r.comments }
func (r *lextwtFile) Twts() types.Twts        { return r.twts }
