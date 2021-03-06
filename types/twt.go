package types

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	TwtHashLength = 7
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrInvalidFeed    = errors.New("erroneous feed detected")
)

type ErrDeadFeed struct {
	Reason string
}

func (e ErrDeadFeed) Error() string {
	return fmt.Sprintf("dead feed: %s", e.Reason)
}

// Twter ...
type Twter struct {
	Nick       string
	URI        string
	HashingURI string

	// URL Deprecated field repalced by URI
	// Remove post Cache v20
	URL string

	Avatar  string
	Tagline string

	Following int
	Followers int

	Follow map[string]Twter

	// Metadata holds additinoal KV pairs (properties) of the feed
	// See: [Metadata Extension](https://dev.twtxt.net/doc/metadataextension.html)
	Metadata url.Values
}

func (twter Twter) IsZero() bool {
	return twter.Nick == "" && twter.URI == ""
}

func (twter Twter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Nick       string `json:"nick"`
		URI        string `json:"uri"`
		HashingURI string `json:"hashing_uri"`
		// URL Deprecated and maintained for backwards compatibility with APIv1
		// Remove in APIv2
		URL       string           `json:"url"`
		Avatar    string           `json:"avatar"`
		Tagline   string           `json:"tagline"`
		Following int              `json:"following"`
		Followers int              `json:"followers"`
		Follow    map[string]Twter `json:"follow"`
	}{
		Nick:       twter.Nick,
		URI:        twter.URI,
		HashingURI: twter.HashingURI,
		URL:        twter.URI,
		Avatar:     twter.Avatar,
		Tagline:    twter.Tagline,
		Following:  twter.Following,
		Followers:  twter.Followers,
		Follow:     twter.Follow,
	})
}

func (twter Twter) String() string { return fmt.Sprintf("%v\t%v", twter.Nick, twter.URI) }

func (twter Twter) Domain() string {
	if sp := strings.SplitN(twter.Nick, "@", 2); len(sp) == 2 {
		return sp[1]
	}
	if url, err := url.Parse(twter.URI); err == nil {
		return url.Hostname()
	}
	return ""
}
func (twter Twter) DomainNick() string {
	if strings.ContainsRune(twter.Nick, '@') {
		return twter.Nick
	}

	if url, err := url.Parse(twter.URI); err == nil {
		return twter.Nick + "@" + url.Hostname()
	}

	return twter.Nick
}

// Twt ...
type Twt interface {
	IsZero() bool
	Clone() Twt

	Twter() Twter
	Created() time.Time

	Hash() string
	Subject() Subject
	Mentions() MentionList
	Links() LinkList
	Tags() TagList

	ExpandMentions(FmtOpts, FeedLookup)

	fmt.Formatter

	// TODO: remove FormatText and add func to update links for UI
	FormatText(TwtTextFormat, FmtOpts) string
}

type TwtMention interface {
	Twter() Twter
}

type MentionList []TwtMention

func (ml MentionList) IsMentioned(twter Twter) bool {
	for _, m := range ml {
		if m.Twter().URI == twter.URI {
			return true
		}
	}
	return false
}

func (ml MentionList) Mentions() []string {
	lis := make([]string, len(ml))
	for i := range ml {
		lis[i] = fmt.Sprint(ml[i])
	}
	return lis
}

type TwtTag interface {
	Text() string
	Target() string
}

type TagList []TwtTag

func (tags *TagList) Tags() []string {
	if tags == nil {
		return nil
	}
	lis := make([]string, len(*tags))
	for i, t := range *tags {
		lis[i] = t.Text()
	}
	return lis
}

type TwtLink interface {
	Text() string
	Target() string
}

type LinkList []TwtLink

func (l LinkList) Links() []string {
	lis := make([]string, len(l))
	for i := range l {
		lis[i] = fmt.Sprint(l[i])
	}
	return lis
}

type Subject interface {
	Text() string
	Tag() TwtTag

	String() string
}

// TwtMap ...
type TwtMap map[string]Twt

// Twts typedef to be able to attach sort methods
type Twts []Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	if twts[i].Created().Before(twts[j].Created()) {
		return false
	}

	if twts[i].Created().After(twts[j].Created()) {
		return true
	}

	return twts[i].Hash() > twts[j].Hash()
}

func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
}

// Tags ...
func (twts Twts) Tags() TagList {
	var tags TagList
	for _, twt := range twts {
		tags = append(tags, twt.Tags()...)
	}
	return tags
}

func (twts Twts) TagCount() map[string]int {
	tags := make(map[string]int)
	for _, twt := range twts {
		for _, tag := range twt.Tags() {
			if txt := tag.Text(); txt != "" {
				tags[txt]++
			} else {
				tags[tag.Target()]++
			}
		}
	}
	return tags
}

// Mentions ...
func (twts Twts) Mentions() MentionList {
	var mentions MentionList
	for _, twt := range twts {
		mentions = append(mentions, twt.Mentions()...)
	}
	return mentions
}
func (twts Twts) MentionCount() map[string]int {
	mentions := make(map[string]int)
	for _, twt := range twts {
		for _, m := range twt.Mentions() {
			mentions[m.Twter().Nick]++
		}
	}
	return mentions
}

// Links ...
func (twts Twts) Links() LinkList {
	var links LinkList
	for _, twt := range twts {
		links = append(links, twt.Links()...)
	}
	return links
}
func (twts Twts) LinkCount() map[string]int {
	links := make(map[string]int)
	for _, twt := range twts {
		for _, link := range twt.Links() {
			links[link.Target()]++
		}
	}
	return links
}

// Subjects ...
func (twts Twts) Subjects() []Subject {
	var subjects []Subject
	for _, twt := range twts {
		subjects = append(subjects, twt.Subject())
	}
	return subjects
}

func (twts Twts) SubjectCount() map[string]int {
	subjects := make(map[string]int)
	for _, twt := range twts {
		subjects[twt.Subject().String()]++
	}
	return subjects
}

func (twts Twts) Clone() Twts {
	lis := make([]Twt, len(twts))
	for i := range twts {
		lis[i] = twts[i].Clone()
	}
	return lis
}

func (twts *Twts) UnmarshalJSON(data []byte) error {
	if twts == nil {
		return fmt.Errorf("uninitialized *Twts")
	}
	var lis []json.RawMessage
	err := json.Unmarshal(data, &lis)
	if err != nil {
		return err
	}

	for _, b := range lis {
		twt, err := DecodeJSON(b)
		if err != nil {
			return err
		}
		*twts = append(*twts, twt)
	}
	return nil
}

type FmtOpts interface {
	LocalURL() *url.URL
	IsLocalURL(string) bool
	UserURL(string) string
	ExternalURL(nick, uri string) string
	URLForTag(tag string) string
	URLForUser(user string) string
}

// TwtTextFormat represents the format of which the twt text gets formatted to
type TwtTextFormat int

const (
	// MarkdownFmt to use markdown format
	MarkdownFmt TwtTextFormat = iota
	// HTMLFmt to use HTML format
	HTMLFmt
	// TextFmt to use for og:description
	TextFmt
	// LiteralFmt is the raw literal format as written/read to/from a feed
	LiteralFmt
)

type nilTwt struct{}

var NilTwt = &nilTwt{}

var _ Twt = NilTwt
var _ gob.GobEncoder = NilTwt
var _ gob.GobDecoder = NilTwt

func (*nilTwt) IsZero() bool { return true }
func (*nilTwt) Clone() Twt   { return NilTwt }

func (*nilTwt) Twter() Twter       { return Twter{} }
func (*nilTwt) Created() time.Time { return time.Now() }
func (*nilTwt) Text() string       { return "" }

func (*nilTwt) Hash() string          { return "" }
func (*nilTwt) Subject() Subject      { return nil }
func (*nilTwt) Mentions() MentionList { return nil }
func (*nilTwt) Tags() TagList         { return nil }
func (*nilTwt) Links() LinkList       { return nil }

func (*nilTwt) ExpandMentions(FmtOpts, FeedLookup)       {}
func (*nilTwt) Format(state fmt.State, c rune)           {}
func (*nilTwt) FormatTwt() string                        { return "" }
func (*nilTwt) FormatText(TwtTextFormat, FmtOpts) string { return "" }

func (*nilTwt) String() string             { return "" }
func (*nilTwt) GobDecode([]byte) error     { return ErrNotImplemented }
func (*nilTwt) GobEncode() ([]byte, error) { return nil, ErrNotImplemented }

func init() {
	gob.Register(&nilTwt{})
}

type TwtManager interface {
	DecodeJSON([]byte) (Twt, error)
	ParseLine(string, *Twter) (Twt, error)
	ParseFile(io.Reader, *Twter) (TwtFile, error)
	MakeTwt(twter Twter, ts time.Time, text string) Twt
}

type nilManager struct{}

var _ TwtManager = nilManager{}

func (nilManager) DecodeJSON([]byte) (Twt, error) { panic("twt managernot configured") }
func (nilManager) ParseLine(line string, twter *Twter) (twt Twt, err error) {
	panic("twt managernot configured")
}
func (nilManager) ParseFile(r io.Reader, twter *Twter) (TwtFile, error) {
	panic("twt managernot configured")
}
func (nilManager) MakeTwt(twter Twter, ts time.Time, text string) Twt {
	panic("twt managernot configured")
}

var twtManager TwtManager = &nilManager{}

func DecodeJSON(b []byte) (Twt, error) { return twtManager.DecodeJSON(b) }
func ParseLine(line string, twter *Twter) (twt Twt, err error) {
	return twtManager.ParseLine(line, twter)
}
func ParseFile(r io.Reader, twter *Twter) (TwtFile, error) {
	return twtManager.ParseFile(r, twter)
}
func MakeTwt(twter Twter, ts time.Time, text string) Twt {
	return twtManager.MakeTwt(twter, ts, text)
}

func SetTwtManager(m TwtManager) {
	twtManager = m
}

type TwtFile interface {
	Twter() *Twter
	Info() Info
	Twts() Twts
}

type Info interface {
	Following() []Twter
	FollowMap() map[string]Twter

	KV
}
type KV interface {
	Values() url.Values
	GetN(string, int) (Value, bool)
	GetAll(string) []Value
	fmt.Stringer
}

type Value interface {
	Key() string
	Value() string
}

// SplitTwts into three groupings.
// The first with created > now (future?! misbehaving clients or misconfigured timezone)
// The first with created > ttl or at most N.
// The third all remaining twts.
func SplitTwts(twts Twts, ttl time.Duration, N int) (Twts, Twts, Twts) {
	now := time.Now()
	oldTime := now.Add(-ttl)

	sort.Sort(twts)

	pos := 0
	for ; pos < len(twts); pos++ {
		if twts[pos].Created().Before(now) {
			break
		}
	}
	future := twts[:pos]
	twts = twts[pos:]

	pos = 0
	for ; pos < len(twts) && pos < N; pos++ {
		if twts[pos].Created().Before(oldTime) {
			break
		}
	}

	return future, twts[:pos], twts[pos:]
}

type FeedLookup interface {
	FeedLookup(string) *Twter
}

type FeedLookupFn func(string) *Twter

func (fn FeedLookupFn) FeedLookup(s string) *Twter { return fn(s) }

func NormalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}
