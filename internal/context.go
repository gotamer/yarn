package internal

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"

	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/internal/session"
	"git.mills.io/yarnsocial/yarn/types"
	"github.com/justinas/nosurf"
	"github.com/theplant-retired/timezones"
)

type Link struct {
	Href string
	Rel  string
}

type Alternative struct {
	Type  string
	Title string
	URL   string
}

type Alternatives []Alternative
type Links []Link

type Meta struct {
	Title       string
	Description string
	UpdatedAt   string
	Image       string
	Author      string
	URL         string
	Keywords    string
}

type Context struct {
	Debug bool

	Logo              template.HTML
	BaseURL           string
	InstanceName      string
	SoftwareVersion   SoftwareConfig
	TwtsPerPage       int
	TwtPrompt         string
	MaxTwtLength      int
	RegisterDisabled  bool
	OpenProfiles      bool
	DisableMedia      bool
	DisableFfmpeg     bool
	WhitelistedImages []string
	BlacklistedFeeds  []string
	EnabledFeatures   []string

	Timezones []*timezones.Zoneinfo

	Reply         string
	Username      string
	User          *User
	LastTwt       types.Twt
	Profile       types.Profile
	Authenticated bool
	IsAdmin       bool

	DisplayDatesInTimezone string
	DisplayTimePreference  string
	OpenLinksInPreference  string

	Error       bool
	Message     string
	Lang        string // language
	AcceptLangs string // accept languages
	Theme       string // not to be confused with the config.Theme
	Commit      string

	Page    string
	Content template.HTML

	Title        string
	Meta         Meta
	Links        Links
	Alternatives Alternatives

	Twter       types.Twter
	Twts        types.Twts
	LocalFeeds  []*Feed
	UserFeeds   []*Feed
	FeedSources FeedSourceMap
	Pager       *paginator.Paginator

	// Time
	TimelineUpdatedAt time.Time
	DiscoverUpdatedAt time.Time
	LastMentionedAt   time.Time

	// Search
	SearchQuery string

	// Tools
	Bookmarklet string

	// Report abuse
	ReportNick string
	ReportURL  string

	// Reset Password Token
	PasswordResetToken string

	// CSRF Token
	CSRFToken string
}

func NewContext(s *Server, req *http.Request) *Context {
	conf := s.config
	db := s.db

	// build logo
	logo, err := RenderLogo(conf.Logo, conf.Name)
	if err != nil {
		log.WithError(err).Error("error rendering logo")
		logo = template.HTML(DefaultLogo)
	}

	// context
	ctx := &Context{
		Debug: conf.Debug,

		Logo:              logo,
		BaseURL:           conf.BaseURL,
		InstanceName:      conf.Name,
		SoftwareVersion:   conf.Version,
		TwtsPerPage:       conf.TwtsPerPage,
		TwtPrompt:         conf.RandomTwtPrompt(),
		MaxTwtLength:      conf.MaxTwtLength,
		RegisterDisabled:  !conf.OpenRegistrations,
		OpenProfiles:      conf.OpenProfiles,
		DisableMedia:      conf.DisableMedia,
		DisableFfmpeg:     conf.DisableFfmpeg,
		LastTwt:           types.NilTwt,
		WhitelistedImages: conf.WhitelistedImages,
		BlacklistedFeeds:  conf.BlacklistedFeeds,
		EnabledFeatures:   conf.Features.AsStrings(),

		DisplayDatesInTimezone: conf.DisplayDatesInTimezone,
		DisplayTimePreference:  conf.DisplayTimePreference,
		OpenLinksInPreference:  conf.OpenLinksInPreference,

		Commit:      yarn.Commit,
		Theme:       conf.Theme,
		Lang:        conf.Lang,
		AcceptLangs: req.Header.Get("Accept-Language"),

		Timezones: timezones.AllZones,

		Title: "",
		Meta: Meta{
			Title:       DefaultMetaTitle,
			Author:      DefaultMetaAuthor,
			Keywords:    DefaultMetaKeywords,
			Description: conf.Description,
		},

		Alternatives: Alternatives{
			Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s local feed", conf.Name),
				URL:   fmt.Sprintf("%s/atom.xml", conf.BaseURL),
			},
		},

		// Assume all users are anonymous (overridden below if Authenticated)
		User: &User{
			DisplayDatesInTimezone: conf.DisplayDatesInTimezone,
			DisplayTimePreference:  conf.DisplayTimePreference,
			OpenLinksInPreference:  conf.OpenLinksInPreference,
		},
		Twter: types.Twter{},

		CSRFToken: nosurf.Token(req),
	}

	if sess := req.Context().Value(session.SessionKey); sess != nil {
		if username, ok := sess.(*session.Session).Get("username"); ok {
			ctx.Authenticated = true
			ctx.Username = username
			user, err := db.GetUser(ctx.Username)
			if err != nil {
				log.WithError(err).Warnf("error loading user object for %s", ctx.Username)
			} else {
				ctx.Twter = types.Twter{
					Nick: user.Username,
					URL:  URLForUser(conf.BaseURL, user.Username),
				}
				ctx.User = user
				ctx.IsAdmin = strings.EqualFold(username, conf.AdminUser)
			}
		}
	}

	// Set the theme based on user preferences
	theme := strings.ToLower(ctx.User.Theme)
	switch theme {
	case "auto":
		ctx.Theme = ""
	case "light", "dark":
		ctx.Theme = theme
	default:
		// Default to the configured theme
		ctx.Theme = conf.Theme
	}
	// Set user language
	lang := strings.ToLower(ctx.User.Lang)
	if lang != "" && lang != "auto" {
		ctx.Lang = lang
	}

	return ctx
}

func (ctx *Context) Translate(translator *Translator, data ...interface{}) {
	// TwtPrompt
	defualtTwtPrompts := translator.Translate(ctx, "DefaultTwtPrompts", data...)
	twtPrompts := strings.Split(defualtTwtPrompts, "\n")
	n := rand.Int() % len(twtPrompts)
	ctx.TwtPrompt = twtPrompts[n]
}
