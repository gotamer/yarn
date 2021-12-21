package types

import (
	"fmt"
	"time"
)

type Follower struct {
	URL           string
	Nick          string
	LastFetchedAt time.Time
}

func (f *Follower) String() string {
	return fmt.Sprintf("@<%s %s>", f.Nick, f.URL)
}

type Followers []*Follower

func (followers Followers) Len() int { return len(followers) }
func (followers Followers) Less(i, j int) bool {
	return followers[i].LastFetchedAt.Before(followers[i].LastFetchedAt)
}
func (followers Followers) Swap(i, j int) { followers[i], followers[j] = followers[j], followers[i] }

// Profile represents a user/feed profile
type Profile struct {
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

	// `true` if the User viewing the Profile has follows this user/feed
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
	Followers  Followers

	// `true` if the User viewing the Profile has permissions to show the
	// bookmarks/followers/followings of this user/feed
	ShowBookmarks bool
	ShowFollowers bool
	ShowFollowing bool
}

type Link struct {
	Title string
	URL   string
}

type Links []Link
