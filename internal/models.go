package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/creasty/defaults"
	log "github.com/sirupsen/logrus"
)

const (
	maxUserFeeds = 5 // 5 is < 7 and humans can only really handle ~7 things
)

var (
	ErrFeedAlreadyExists = errors.New("error: feed already exists by that name")
	ErrAlreadyFollows    = errors.New("error: you already follow this feed")
	ErrTooManyFeeds      = errors.New("error: you have too many feeds")
)

// Feed ...
type Feed struct {
	Name        string
	Description string
	URL         string
	CreatedAt   time.Time

	AvatarHash string `defaulf:""`

	Followers map[string]string `default:"{}"`

	remotes map[string]string
}

// User ...
type User struct {
	Username   string
	Password   string
	Tagline    string
	URL        string
	CreatedAt  time.Time
	LastSeenAt time.Time

	Theme                      string `default:"auto"`
	Lang                       string `default:""`
	Recovery                   string `default:""`
	AvatarHash                 string `default:""`
	DisplayDatesInTimezone     string `default:"UTC"`
	DisplayTimePreference      string `default:"24h"`
	OpenLinksInPreference      string `default:"newwindow"`
	IsFollowersPubliclyVisible bool   `default:"true"`
	IsFollowingPubliclyVisible bool   `default:"true"`
	IsBookmarksPubliclyVisible bool   `default:"true"`

	Feeds []string `default:"[]"`

	Bookmarks map[string]string `default:"{}"`
	Followers map[string]string `default:"{}"`
	Following map[string]string `default:"{}"`
	Muted     map[string]string `default:"{}"`

	muted   map[string]string
	remotes map[string]string
	sources map[string]string
}

func CreateFeed(conf *Config, db Store, user *User, name string, force bool) error {
	if user != nil {
		if !force && len(user.Feeds) > maxUserFeeds {
			return ErrTooManyFeeds
		}
	}

	fn := filepath.Join(conf.Data, feedsDir, name)
	stat, err := os.Stat(fn)

	if err == nil && !force {
		return ErrFeedAlreadyExists
	}

	if stat == nil {
		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			return err
		}
	}

	if user != nil {
		if !user.OwnsFeed(name) {
			user.Feeds = append(user.Feeds, name)
		}
	}

	followers := make(map[string]string)
	if user != nil {
		followers[user.Username] = user.URL
	}

	feed := NewFeed()
	feed.Name = name
	feed.URL = URLForUser(conf.BaseURL, name)
	feed.Followers = followers
	feed.CreatedAt = time.Now()

	if err := db.SetFeed(name, feed); err != nil {
		return err
	}

	if user != nil {
		user.Follow(name, feed.URL)
	}

	return nil
}

func DetachFeedFromOwner(db Store, user *User, feed *Feed) (err error) {
	delete(user.Following, feed.Name)
	delete(user.sources, feed.URL)

	user.Feeds = RemoveString(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
		return
	}

	delete(feed.Followers, user.Username)
	if err = db.SetFeed(feed.Name, feed); err != nil {
		return
	}

	return nil
}

func RemoveFeedOwnership(db Store, user *User, feed *Feed) (err error) {
	user.Feeds = RemoveString(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
		return
	}

	return nil
}

func AddFeedOwnership(db Store, user *User, feed *Feed) (err error) {
	user.Feeds = append(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
		return
	}

	return nil
}

// NewFeed ...
func NewFeed() *Feed {
	feed := &Feed{}
	if err := defaults.Set(feed); err != nil {
		log.WithError(err).Error("error creating new feed object")
	}
	return feed
}

// LoadFeed ...
func LoadFeed(data []byte) (feed *Feed, err error) {
	feed = &Feed{}
	if err := defaults.Set(feed); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	if feed.Followers == nil {
		feed.Followers = make(map[string]string)
	}

	feed.remotes = make(map[string]string)
	for n, u := range feed.Followers {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		feed.remotes[u] = n
	}

	return
}

// NewUser ...
func NewUser() *User {
	user := &User{}
	if err := defaults.Set(user); err != nil {
		log.WithError(err).Error("error creating new user object")
	}
	user.muted = make(map[string]string)
	user.remotes = make(map[string]string)
	user.sources = make(map[string]string)
	return user
}

func LoadUser(data []byte) (user *User, err error) {
	user = &User{}
	if err := defaults.Set(user); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	if user.Bookmarks == nil {
		user.Bookmarks = make(map[string]string)
	}
	if user.Followers == nil {
		user.Followers = make(map[string]string)
	}
	if user.Following == nil {
		user.Following = make(map[string]string)
	}

	user.muted = make(map[string]string)
	for n, u := range user.Muted {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.muted[u] = n
	}

	user.remotes = make(map[string]string)
	for n, u := range user.Followers {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.remotes[u] = n
	}

	user.sources = make(map[string]string)
	for n, u := range user.Following {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.sources[u] = n
	}

	return
}

func (f *Feed) AddFollower(nick, uri string) {
	uri = NormalizeURL(uri)
	if _, ok := f.Followers[nick]; ok {
		if _u, err := url.Parse(uri); err == nil {
			nick = fmt.Sprintf("%s@%s", nick, _u.Hostname())
		} else {
			nick = UniqueKeyFor(f.Followers, nick)
		}
	}
	f.Followers[nick] = uri
	f.remotes[uri] = nick
}

func (f *Feed) FollowedBy(url string) bool {
	_, ok := f.remotes[NormalizeURL(url)]
	return ok
}

func (f *Feed) Source() types.Feeds {
	feeds := make(types.Feeds)
	feeds[types.Feed{Nick: f.Name, URL: f.URL}] = true
	return feeds
}

func (f *Feed) Profile(baseURL string, viewer *User) types.Profile {
	var (
		follows    bool
		followedBy bool
		muted      bool
	)

	if viewer != nil {
		follows = viewer.Follows(f.URL)
		followedBy = viewer.FollowedBy(f.URL)
		muted = viewer.HasMuted(f.URL)
	}

	return types.Profile{
		Type: "Feed",

		Nick:        f.Name,
		Description: f.Description,
		URI:         f.URL,
		Avatar:      URLForAvatar(baseURL, f.Name, f.AvatarHash),

		Follows:    follows,
		FollowedBy: followedBy,
		Muted:      muted,

		ShowBookmarks: false, // feeds don't have bookmarks
		ShowFollowers: true,  // feeds can't control this
		ShowFollowing: false, // feeds can't follow others
	}
}

func (f *Feed) Twter(conf *Config) types.Twter {
	return types.Twter{
		Nick:   f.Name,
		URI:    conf.URLForUser(f.Name),
		Avatar: conf.URLForAvatar(f.Name, f.AvatarHash),
	}
}

func (f *Feed) Bytes() ([]byte, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (u *User) String() string {
	url, err := url.Parse(u.URL)
	if err != nil {
		log.WithError(err).Warn("error parsing user url")
		return u.Username
	}
	return fmt.Sprintf("%s@%s", u.Username, url.Hostname())
}

func (u *User) IsZero() bool {
	return u.Username == ""
}

func (u *User) OwnsFeed(name string) bool {
	name = NormalizeFeedName(name)
	for _, feed := range u.Feeds {
		if NormalizeFeedName(feed) == name {
			return true
		}
	}
	return false
}

func (u *User) Is(url string) bool {
	return u.URL == NormalizeURL(url)
}

func (u *User) Bookmark(hash string) {
	if _, ok := u.Bookmarks[hash]; !ok {
		u.Bookmarks[hash] = ""
	} else {
		delete(u.Bookmarks, hash)
	}
}

func (u *User) Bookmarked(hash string) bool {
	_, ok := u.Bookmarks[hash]
	return ok
}

func (u *User) AddFollower(nick, uri string) {
	uri = NormalizeURL(uri)
	if _, ok := u.Followers[nick]; ok {
		if _u, err := url.Parse(uri); err == nil {
			nick = fmt.Sprintf("%s@%s", nick, _u.Hostname())
		} else {
			nick = UniqueKeyFor(u.Followers, nick)
		}
	}
	u.Followers[nick] = uri
	u.remotes[uri] = nick
}

func (u *User) FollowedBy(url string) bool {
	_, ok := u.remotes[NormalizeURL(url)]
	return ok
}

func (u *User) Mute(nick, url string) {
	if !u.HasMuted(url) {
		u.Muted[nick] = url
		u.muted[url] = nick
	}
}

func (u *User) Unmute(nick string) {
	url, ok := u.Muted[nick]
	if ok {
		delete(u.Muted, nick)
		delete(u.muted, url)
	}
}

func (u *User) Follow(alias, uri string) error {
	if !u.Follows(uri) {
		if _, ok := u.Following[alias]; ok {
			if _u, err := url.Parse(uri); err == nil {
				alias = fmt.Sprintf("%s@%s", alias, _u.Hostname())
			} else {
				alias = UniqueKeyFor(u.Following, alias)
			}
		}

		u.Following[alias] = uri
		u.sources[uri] = alias
	}
	return nil
}

func (u *User) FollowAndValidate(conf *Config, alias, uri string) error {
	tf, err := ValidateFeed(conf, alias, uri)
	if err != nil {
		return err
	}
	twter := tf.Twter()

	if u.Follows(twter.URI) {
		return ErrAlreadyFollows
	}

	// If no nick provided try to guess a suitable nick
	// from the feed or some heuristics from the feed's URI
	// (borrowed from Yarns)
	if alias == "" {
		if twter.Nick != "" {
			alias = twter.Nick
		} else {
			// TODO: Move this logic into types/lextwt and types/retwt
			if u, err := url.Parse(uri); err == nil {
				if strings.HasSuffix(u.Path, "/twtxt.txt") {
					if rest := strings.TrimSuffix(u.Path, "/twtxt.txt"); rest != "" {
						alias = strings.Trim(rest, "/")
					} else {
						alias = u.Hostname()
					}
				} else if strings.HasSuffix(u.Path, ".txt") {
					base := filepath.Base(u.Path)
					if name := strings.TrimSuffix(base, filepath.Ext(base)); name != "" {
						alias = name
					} else {
						alias = u.Hostname()
					}
				} else {
					alias = Slugify(uri)
				}
			}
		}
	}

	return u.Follow(alias, twter.URI)
}

func (u *User) Follows(url string) bool {
	_, ok := u.sources[NormalizeURL(url)]
	return ok
}

func (u *User) FollowsAs(url string) string {
	if url, ok := u.sources[NormalizeURL(url)]; ok {
		return url
	}
	return ""
}

func (u *User) Unfollow(alias string) {
	if url, ok := u.Following[alias]; ok {
		delete(u.sources, url)
		delete(u.Following, alias)
	}
}

func (u *User) HasMuted(url string) bool {
	_, ok := u.muted[NormalizeURL(url)]
	return ok
}

func (u *User) Source() types.Feeds {
	feeds := make(types.Feeds)
	feeds[types.Feed{Nick: u.Username, URL: u.URL}] = true
	return feeds
}

func (u *User) Sources() types.Feeds {
	// Ensure we fetch the user's own posts in the cache
	feeds := u.Source()
	for url, nick := range u.sources {
		feeds[types.Feed{Nick: nick, URL: url}] = true
	}
	return feeds
}

func (u *User) Profile(baseURL string, viewer *User) types.Profile {
	var (
		feeds            []string
		viewerFollows    bool
		followedByViewer bool
		muted            bool
		showBookmarks    bool
		showFollowers    bool
		showFollowing    bool
	)

	if viewer != nil {
		if viewer.Is(u.URL) {
			feeds = u.Feeds
			viewerFollows = true
			followedByViewer = true
			showBookmarks = true
			showFollowers = true
			showFollowing = true
		} else {
			viewerFollows = viewer.Follows(u.URL)
			followedByViewer = viewer.FollowedBy(u.URL)
			showBookmarks = u.IsBookmarksPubliclyVisible
			showFollowers = u.IsFollowersPubliclyVisible
			showFollowing = u.IsFollowingPubliclyVisible
		}

		muted = viewer.HasMuted(u.URL)
	}

	var follows types.Follows

	if showFollowing {
		for nick, uri := range u.Following {
			follows = append(follows, types.Follow{Nick: nick, URI: uri})
		}
	}

	return types.Profile{
		Type: "User",

		Nick:        u.Username,
		Description: u.Tagline,
		URI:         URLForUser(baseURL, u.Username),
		Avatar:      URLForAvatar(baseURL, u.Username, u.AvatarHash),

		Follows:    viewerFollows,
		FollowedBy: followedByViewer,
		Muted:      muted,
		Feeds:      feeds,

		Bookmarks: u.Bookmarks,

		Following:  follows,
		NFollowing: len(follows),

		ShowBookmarks: showBookmarks,
		ShowFollowers: showFollowers,
		ShowFollowing: showFollowing,

		LastSeenAt: u.LastSeenAt,
	}
}

func (u *User) Twter(conf *Config) types.Twter {
	return types.Twter{
		Nick:   u.Username,
		URI:    conf.URLForUser(u.Username),
		Avatar: conf.URLForAvatar(u.Username, u.AvatarHash),
	}
}

func (u *User) Filter(twts []types.Twt) (filtered []types.Twt) {
	// fast-path
	if len(u.muted) == 0 {
		return twts
	}

	filtered = make([]types.Twt, 0)
	for _, twt := range twts {
		if u.HasMuted(twt.Twter().URI) {
			continue
		}
		filtered = append(filtered, twt)
	}
	return
}

func (u *User) Reply(twt types.Twt) string {
	// Initialise the list of tokens with the twt's Subject
	tokens := []string{twt.Subject().String()}

	// If we follow the original twt's Twter, add them as the first mention
	// only if the original twter isn't ourselves!
	if u.Follows(twt.Twter().URI) && !u.Is(twt.Twter().URI) {
		tokens = append(tokens, fmt.Sprintf("@%s", u.FollowsAs(twt.Twter().URI)))
	}

	return fmt.Sprintf("%s ", strings.Join(tokens, " "))
}

func (u *User) Fork(twt types.Twt) string {
	// Initialise the list of tokens with the twt's Hash (forking from)
	tokens := []string{fmt.Sprintf("(#%s)", twt.Hash())}

	// If we follow the original twt's Twter, add them as the first mention
	// only if the original twter isn't ourselves!
	if u.Follows(twt.Twter().URI) && !u.Is(twt.Twter().URI) {
		tokens = append(tokens, fmt.Sprintf("@%s", twt.Twter().Nick))
	}

	return fmt.Sprintf("%s ", strings.Join(tokens, " "))
}

func (u *User) DisplayTimeFormat() string {
	switch strings.ToLower(u.DisplayTimePreference) {
	case "12h":
		return "3:04PM"
	case "24h":
		return "15:04"
	default:
		return ""

	}
}

func (u *User) Bytes() ([]byte, error) {
	data, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	return data, nil
}
