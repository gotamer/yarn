package types

import (
	"fmt"
	"time"
)

type Follower struct {
	URI        string
	URL        string
	Nick       string
	LastSeenAt time.Time
}

func (f *Follower) String() string {
	// XXX: Backwards compatibility with old `Followers` struct.
	// TODO: Remove post v0.12.00
	if f.URI == "" {
		f.URI = f.URL
	}

	return fmt.Sprintf("@<%s %s>", f.Nick, f.URI)
}

type Followers []*Follower

func (fs Followers) Len() int { return len(fs) }
func (fs Followers) Less(i, j int) bool {
	return fs[i].LastSeenAt.Before(fs[j].LastSeenAt)
}
func (fs Followers) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}

// AsMap returns the Followers as a map of `nick -> uri` for use by the /whoFollows resource
// which implements the MultiUserAegent spec
// See: https://dev.twtxt.net/doc/useragentextension.html
func (fs Followers) AsMap() map[string]string {
	kv := make(map[string]string)
	for _, f := range fs {
		kv[f.Nick] = f.URI
	}
	return kv
}

type Follow struct {
	URI           string
	Nick          string
	Failures      int
	LastFetchedAt time.Time
	LastSuccessAt time.Time
	LastFailureAt time.Time
}

func (f Follow) String() string {
	return fmt.Sprintf("@<%s %s>", f.Nick, f.URI)
}

type Follows []Follow

func (fs Follows) Len() int { return len(fs) }
func (fs Follows) Less(i, j int) bool {
	return fs[i].Failures < fs[j].Failures
}
func (fs Follows) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}

// OldProfile is a backwards compatible version of `Profile` for APIv1 client compatibility
// such as `yarnc` and the Mobile App that uses a map of `nick -> url` for Followers and
// Followings.
// TODO: Upgrade APIv1 clients
// TODOL Remove this when the Mobile App has been upgraded.
type OldProfile struct {
	Type string

	URL string
	// TODO: Rename to Nick
	Username string
	Avatar   string
	// TODO: Rename to Description
	Tagline string

	Links Links

	// Used by the Mobile App for "Post as..."
	Feeds []string

	// `true` if the User viewing the Profile has muted this user/feed
	Muted bool

	// `true` if the User viewing the Profile follows this user/feed
	Follows bool

	// `true` if user/feed follows the User viewing the Profile.
	FollowedBy bool

	// Timestamp of the profile's last Twt
	LastPostedAt time.Time

	// Timestamp of the profile's last activity (last seen) accurate to a day
	LastSeenAt time.Time

	Bookmarks map[string]string

	NFollowing int
	Following  map[string]string

	NFollowers int
	// TODO: Maybe migrate to use `Followers` type
	// XXX: But be aware doing so breaks API compat
	Followers map[string]string

	// `true` if the User viewing the Profile has permissions to show the
	// bookmarks/followers/followings of this user/feed
	ShowBookmarks bool
	ShowFollowers bool
	ShowFollowing bool
}

// Profile represents a user/feed profile
type Profile struct {
	Type string

	URI         string
	Nick        string
	Avatar      string
	Description string

	Links Links

	// Used by the Mobile App for "Post as..."
	Feeds []string

	// `true` if the User viewing the Profile has muted this user/feed
	Muted bool

	// `true` if the User viewing the Profile follows this user/feed
	Follows bool

	// `true` if user/feed follows the User viewing the Profile.
	FollowedBy bool

	// Timestamp of the profile's last Twt
	LastPostedAt time.Time

	// Timestamp of the profile's last activity (last seen) accurate to a day
	LastSeenAt time.Time

	Bookmarks map[string]string

	NFollowing int
	Following  Follows

	NFollowers int
	Followers  Followers

	// `true` if the User viewing the Profile has permissions to show the
	// bookmarks/followers/followings of this user/feed
	ShowBookmarks bool
	ShowFollowers bool
	ShowFollowing bool
}

// AsOldProfile returns a `Profilev1` object for compatibility with APIv1 clients
// such as the Mobile App.
// TODO: Remove when Mobile App is upgraded
func (p Profile) AsOldProfile() OldProfile {
	followingKV := make(map[string]string)
	followersKV := make(map[string]string)

	for _, following := range p.Following {
		followingKV[following.Nick] = following.URI
	}

	for _, follower := range p.Followers {
		followersKV[follower.Nick] = follower.URI
	}

	return OldProfile{
		Type: p.Type,

		URL:      p.URI,
		Username: p.Nick,
		Avatar:   p.Avatar,
		Tagline:  p.Description,

		Links: p.Links,
		Feeds: p.Feeds,

		Muted:      p.Muted,
		Follows:    p.Follows,
		FollowedBy: p.FollowedBy,

		LastPostedAt: p.LastPostedAt,
		LastSeenAt:   p.LastSeenAt,

		Bookmarks: p.Bookmarks,

		NFollowing: p.NFollowing,
		Following:  followingKV,

		NFollowers: p.NFollowers,
		Followers:  followersKV,

		ShowBookmarks: p.ShowBookmarks,
		ShowFollowers: p.ShowFollowers,
		ShowFollowing: p.ShowFollowing,
	}
}

type Link struct {
	Title string
	URL   string
}

type Links []Link
