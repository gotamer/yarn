package types

// Profile represents a user/feed profile
type Profile struct {
	Type string

	URL string
	// TODO: Rename to Nick
	Username string
	Avatar   string
	// TODO: Rename to Description
	Tagline string

	// TODO: Replace with Links []Link
	BlogsURL string
	Links    Links

	// `true` if the User viewing the Profile has muted this user/feed
	Muted bool

	// `true` if the User viewing the Profile has follows this user/feed
	Follows bool

	// `true` if user/feed follows the User viewing the Profile.
	FollowedBy bool

	Bookmarks map[string]string
	Followers map[string]string
	Following map[string]string

	NFollowers int
	NFollowing int

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
