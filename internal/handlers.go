package internal

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gorilla/feeds"
	"github.com/james4k/fmatter"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
	"gopkg.in/yaml.v2"

	"git.mills.io/yarnsocial/yarn/internal/session"
	"git.mills.io/yarnsocial/yarn/types"
)

const (
	MediaResolution  = 720 // 720x576
	AvatarResolution = 360 // 360x360
	AsyncTaskLimit   = 5
	MaxFailedLogins  = 3 // By default 3 failed login attempts per 5 minutes

	bookmarkletTemplate = `(function(){window.location.href="%s/?title="+document.title+"&url="+document.URL;})();`
)

var (
	ErrFeedImposter = errors.New("error: imposter detected, you do not own this feed")
)

//go:embed pages/*.md
var pages embed.FS

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "Endpoint Not Found", http.StatusNotFound)
		return
	}

	ctx := NewContext(s, r)
	ctx.Title = s.tr(ctx, "PageNotFoundTitle")
	w.WriteHeader(http.StatusNotFound)
	s.render("404", w, ctx)
}

type FrontMatter struct {
	Title string
}

// PageHandler ...
func (s *Server) PageHandler(name string) httprouter.Handle {

	var mdTpl string
	if b, err := pages.ReadFile(fmt.Sprintf("pages/%s.md", name)); err == nil {
		mdTpl = string(b)
	} else {
		log.WithError(err).Errorf("error finding page %s", name)
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		md, err := RenderHTML(mdTpl, ctx)
		if err != nil {
			log.WithError(err).Errorf("error rendering page %s", name)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorRenderingPage")
			s.render("error", w, ctx)
			return
		}

		var frontmatter FrontMatter
		content, err := fmatter.Read([]byte(md), &frontmatter)
		if err != nil {
			log.WithError(err).Error("error parsing front matter")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingPage")
			s.render("error", w, ctx)
			return
		}

		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags
		opts := html.RendererOptions{
			Flags:     htmlFlags,
			Generator: "",
		}
		renderer := html.NewRenderer(opts)

		html := markdown.ToHTML(content, p, renderer)

		var title string

		if frontmatter.Title != "" {
			title = frontmatter.Title
		} else {
			title = strings.Title(name)
		}
		ctx.Title = title

		ctx.Page = name
		ctx.Content = template.HTML(html)

		s.render("page", w, ctx)
	}
}

// UserConfigHandler ...
func (s *Server) UserConfigHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick = NormalizeUsername(nick)

		var (
			url       string
			following map[string]string
			bookmarks map[string]string
		)

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			url = user.URL
			if ctx.Authenticated || user.IsFollowingPubliclyVisible {
				following = user.Following
			}
			if ctx.Authenticated || user.IsBookmarksPubliclyVisible {
				bookmarks = user.Bookmarks
			}
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			url = feed.URL
		} else {
			http.Error(w, "User or Feed not found", http.StatusNotFound)
			return
		}

		config := struct {
			Nick      string            `json:"nick"`
			URL       string            `json:"url"`
			Following map[string]string `json:"following"`
			Bookmarks map[string]string `json:"bookmarks"`
		}{
			Nick:      nick,
			URL:       url,
			Following: following,
			Bookmarks: bookmarks,
		}

		data, err := yaml.Marshal(config)
		if err != nil {
			log.WithError(err).Errorf("error exporting user/feed config")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/yaml")
		if r.Method == http.MethodHead {
			return
		}

		_, _ = w.Write(data)
	}
}

// ProfileHandler ...
func (s *Server) ProfileHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoUser")
			s.render("error", w, ctx)
			return
		}

		var profile types.Profile

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingProfile")
				s.render("error", w, ctx)
				return
			}
			profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingProfile")
				s.render("error", w, ctx)
				return
			}
			profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserOrFeedNotFound")
			s.render("404", w, ctx)
			return
		}

		ctx.Profile = profile

		ctx.Links = append(ctx.Links, Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(profile.URL)),
			Rel:  "webmention",
		})

		ctx.Alternatives = append(ctx.Alternatives, Alternatives{
			Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Username),
				URL:   profile.URL,
			},
			Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Username),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URL)),
			},
		}...)

		twts := FilterTwts(ctx.User, s.cache.GetByURL(profile.URL))

		if len(twts) > 0 {
			profile.LastPostedAt = twts[0].Created()
		}

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingTimeline")
			s.render("error", w, ctx)
			return
		}

		ctx.Title = fmt.Sprintf("%s's Profile: %s", profile.Username, profile.Tagline)
		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		s.render("profile", w, ctx)
	}
}

// ManageFeedHandler...
func (s *Server) ManageFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		feedName := NormalizeFeedName(p.ByName("name"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoFeed")
			s.render("error", w, ctx)
			return
		}

		feed, err := s.db.GetFeed(feedName)
		if err != nil {
			log.WithError(err).Errorf("error loading feed object for %s", feedName)
			ctx.Error = true
			if err == ErrFeedNotFound {
				ctx.Message = s.tr(ctx, "ErrorFeedNotFound")
				s.render("404", w, ctx)
			}

			ctx.Message = s.tr(ctx, "ErrorGetFeed")
			s.render("error", w, ctx)
			return
		}

		if !ctx.User.OwnsFeed(feed.Name) {
			ctx.Error = true
			s.render("401", w, ctx)
			return
		}

		trdata := map[string]interface{}{}
		switch r.Method {
		case http.MethodGet:
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
			trdata["Feed"] = feed.Name
			ctx.Title = s.tr(ctx, "PageManageFeedTitle", trdata)
			s.render("manageFeed", w, ctx)
			return
		case http.MethodPost:
			description := r.FormValue("description")
			feed.Description = description

			avatarFile, _, err := r.FormFile("avatar_file")
			if err != nil && err != http.ErrMissingFile {
				log.WithError(err).Error("error parsing form file")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if avatarFile != nil {
				opts := &ImageOptions{
					Resize: true,
					Width:  AvatarResolution,
					Height: AvatarResolution,
				}
				_, err = StoreUploadedImage(
					s.config, avatarFile,
					avatarsDir, feedName,
					opts,
				)
				if err != nil {
					ctx.Error = true
					trdata["Error"] = err.Error()
					ctx.Message = s.tr(ctx, "ErrorUpdateFeed", trdata)
					s.render("error", w, ctx)
					return
				}
				avatarFn := filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.png", feedName))
				if avatarHash, err := FastHashFile(avatarFn); err == nil {
					feed.AvatarHash = avatarHash
				} else {
					log.WithError(err).Warnf("error updating avatar hash for %s", feedName)
				}
			}

			if err := s.db.SetFeed(feed.Name, feed); err != nil {
				log.WithError(err).Warnf("error updating user object for followee %s", feed.Name)

				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorSetFeed")
				s.render("error", w, ctx)
				return
			}

			ctx.Error = false
			ctx.Message = s.tr(ctx, "MsgUpdateFeedSuccess")
			s.render("error", w, ctx)
			return
		}

		ctx.Error = true
		ctx.Message = s.tr(ctx, "ErrorFeedNotFound")
		s.render("404", w, ctx)
	}
}

// ArchiveFeedHandler...
func (s *Server) ArchiveFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		feedName := NormalizeFeedName(p.ByName("name"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoFeed")
			s.render("error", w, ctx)
			return
		}

		feed, err := s.db.GetFeed(feedName)
		if err != nil {
			log.WithError(err).Errorf("error loading feed object for %s", feedName)
			ctx.Error = true
			if err == ErrFeedNotFound {
				ctx.Message = s.tr(ctx, "ErrorFeedNotFound")
				s.render("404", w, ctx)
			}

			ctx.Message = s.tr(ctx, "ErrorLoadingFeed")
			s.render("error", w, ctx)
			return
		}

		if !ctx.User.OwnsFeed(feed.Name) {
			ctx.Error = true
			s.render("401", w, ctx)
			return
		}

		if err := DetachFeedFromOwner(s.db, ctx.User, feed); err != nil {
			log.WithError(err).Warnf("Error detaching feed owner %s from feed %s", ctx.User.Username, feed.Name)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorArchivingFeed")
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgArchiveFeedSuccess")
		s.render("error", w, ctx)
	}
}

// AvatarHandler ...
func (s *Server) AvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if !s.db.HasUser(nick) && !FeedExists(s.config, nick) {
			http.Error(w, "User or Feed Not Found", http.StatusNotFound)
			return
		}

		fn := filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.png", nick))
		w.Header().Set("Content-Type", "image/png")

		if fileInfo, err := os.Stat(fn); err == nil {
			etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))

			if match := r.Header.Get("If-None-Match"); match != "" {
				if strings.Contains(match, etag) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			w.Header().Set("Etag", etag)
			if r.Method == http.MethodHead {
				return
			}

			f, err := os.Open(fn)
			if err != nil {
				log.WithError(err).Error("error opening avatar file")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			if _, err := io.Copy(w, f); err != nil {
				log.WithError(err).Error("error writing avatar response")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			return
		}

		etag := fmt.Sprintf("W/\"%s\"", r.RequestURI)

		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Etag", etag)
		if r.Method == http.MethodHead {
			return
		}

		img, err := GenerateAvatar(s.config, nick)
		if err != nil {
			log.WithError(err).Errorf("error generating avatar for %s", nick)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			return
		}

		// Support older browsers like IE11 that don't support WebP :/
		metrics.Counter("media", "old_avatar").Inc()
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			log.WithError(err).Error("error encoding auto generated avatar")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// PostHandler ...
func (s *Server) PostHandler() httprouter.Handle {
	isLocalURL := IsLocalURLFactory(s.config)
	isExternalFeed := IsExternalFeedFactory(s.config)
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))

		// TODO: Support deleting/patching last feed (`postas`) twt too.
		if r.Method == http.MethodDelete || r.Method == http.MethodPatch {
			if err := DeleteLastTwt(s.config, ctx.User); err != nil {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorDeleteLastTwt")
				s.render("error", w, ctx)
			}

			// TODO: Make this a Task?
			s.tasks.DispatchFunc(func() error {
				// Update user's own timeline with their own new post.
				s.cache.FetchTwts(s.config, s.archive, ctx.User.Source(), nil)

				// Re-populate/Warm cache for User
				s.cache.GetByUser(ctx.User, true)

				return nil
			})

			if r.Method != http.MethodDelete {
				return
			}
		}

		hash := r.FormValue("hash")
		lastTwt, _, err := GetLastTwt(s.config, ctx.User)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeleteLastTwt")
			s.render("error", w, ctx)
			return
		}

		if hash != "" && lastTwt.Hash() == hash {
			if err := DeleteLastTwt(s.config, ctx.User); err != nil {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorDeleteLastTwt")
				s.render("error", w, ctx)
			}
		} else {
			log.Warnf("hash mismatch %s != %s", lastTwt.Hash(), hash)
		}

		text := CleanTwt(r.FormValue("text"))

		if text == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoPostContent")
			s.render("error", w, ctx)
			return
		}

		reply := strings.TrimSpace(r.FormValue("reply"))
		if reply != "" {
			re := regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)
			match := re.FindStringSubmatch(text)
			if match == nil {
				text = fmt.Sprintf("(%s) %s", reply, text)
			}
		}

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorPostingTwt")
			s.render("error", w, ctx)
			return
		}

		var sources types.Feeds
		var twt types.Twt = types.NilTwt

		switch postas {
		case "", user.Username:
			sources = user.Source()

			if hash != "" && lastTwt.Hash() == hash {
				twt, err = AppendTwt(s.config, s.db, user, text, lastTwt.Created())
			} else {
				twt, err = AppendTwt(s.config, s.db, user, text)
			}
		default:
			if user.OwnsFeed(postas) {
				if feed, err := s.db.GetFeed(postas); err == nil {
					sources = feed.Source()
				} else {
					log.WithError(err).Error("error loading feed object")
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorPostingTwt")
					s.render("error", w, ctx)
					return
				}

				if hash != "" && lastTwt.Hash() == hash {
					twt, err = AppendSpecial(s.config, s.db, postas, text, lastTwt.Created)
				} else {
					twt, err = AppendSpecial(s.config, s.db, postas, text)
				}
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			log.WithError(err).Error("error posting twt")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorPostingTwt")
			s.render("error", w, ctx)
			return
		}

		// TODO: Make this a Task?
		s.tasks.DispatchFunc(func() error {
			// Update user's own timeline with their own new post.
			s.cache.FetchTwts(s.config, s.archive, sources, nil)

			// Re-populate/Warm cache for User
			s.cache.GetByUser(ctx.User, true)

			return nil
		})

		// WebMentions ...
		// TODO: Use a queue here instead?
		if _, err := s.tasks.Dispatch(NewFuncTask(func() error {
			for _, m := range twt.Mentions() {
				twter := m.Twter()
				if !isLocalURL(twter.URL) || isExternalFeed(twter.URL) {
					if err := WebMention(twter.URL, URLForTwt(s.config.BaseURL, twt.Hash())); err != nil {
						log.WithError(err).Warnf("error sending webmention to %s", twter.URL)
					}
				}
			}
			return nil
		})); err != nil {
			log.WithError(err).Warn("error submitting task for webmentions")
		}

		http.Redirect(w, r, RedirectRefererURL(r, s.config, "/"), http.StatusFound)
	}
}

// WebMentionHandler ...
func (s *Server) WebMentionHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		webmentions.WebMentionEndpoint(w, r)
	}
}

// FeedHandler ...
func (s *Server) FeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		name := NormalizeFeedName(r.FormValue("name"))
		trdata := map[string]interface{}{}
		if err := ValidateFeedName(s.config.Data, name); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorInvalidFeedName", trdata)
			s.render("error", w, ctx)
			return
		}

		if err := CreateFeed(s.config, s.db, ctx.User, name, false); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorCreateFeed", trdata)
			s.render("error", w, ctx)
			return
		}

		ctx.User.Follow(name, URLForUser(s.config.BaseURL, name))

		if err := s.db.SetUser(ctx.Username, ctx.User); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorCreateFeed", trdata)
			s.render("error", w, ctx)
			return
		}

		if _, err := AppendSpecial(
			s.config, s.db,
			twtxtBot,
			fmt.Sprintf(
				"FEED: @<%s %s> from @<%s %s>",
				name, URLForUser(s.config.BaseURL, name),
				ctx.User.Username, URLForUser(s.config.BaseURL, ctx.User.Username),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special FOLLOW post")
		}

		ctx.Error = false
		trdata["Feed"] = name
		ctx.Message = s.tr(ctx, "MsgCreateFeedSuccess", trdata)
		s.render("error", w, ctx)

	}
}

// FeedsHandler ...
func (s *Server) FeedsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)
		user := ctx.User

		allFeeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingFeeds")
			s.render("error", w, ctx)
			return
		}

		feedSources, err := LoadFeedSources(s.config.Data)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingFeeds")
			s.render("error", w, ctx)
			return
		}

		var (
			userFeeds  []*Feed
			localFeeds []*Feed
		)

		for _, feed := range allFeeds {
			if user.OwnsFeed(feed.Name) {
				userFeeds = append(userFeeds, feed)
			} else {
				localFeeds = append(localFeeds, feed)
			}
		}

		ctx.Title = s.tr(ctx, "PageFeedsTitle")
		ctx.LocalFeeds = localFeeds
		ctx.UserFeeds = userFeeds
		ctx.FeedSources = feedSources.Sources

		s.render("feeds", w, ctx)
	}
}

// LoginHandler ...
func (s *Server) LoginHandler() httprouter.Handle {
	// #239: Throttle failed login attempts and lock user  account.
	failures := NewTTLCache(5 * time.Minute)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			s.render("login", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		rememberme := r.FormValue("rememberme") == "on"

		// Error: no username or password provided
		if username == "" || password == "" {
			log.Warn("no username or password provided")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Lookup user
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidUsername")
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		if failures.Get(user.Username) > MaxFailedLogins {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorMaxFailedLogins")
			s.render("error", w, ctx)
			return
		}

		// Validate cleartext password against KDF hash
		err = s.pm.CheckPassword(user.Password, password)
		if err != nil {
			// #239: Throttle failed login attempts and lock user  account.
			failed := failures.Inc(user.Username)
			time.Sleep(time.Duration(IntPow(2, failed)) * time.Second)

			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidPassword")
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		failures.Reset(user.Username)

		// Login successful
		log.Infof("login successful: %s", username)

		// Lookup session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			log.Warn("no session found")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Authorize session
		_ = sess.(*session.Session).Set("username", username)

		// Persist session?
		if rememberme {
			_ = sess.(*session.Session).Set("persist", "1")
		}

		http.Redirect(w, r, RedirectRefererURL(r, s.config, "/"), http.StatusFound)
	}
}

// LogoutHandler ...
func (s *Server) LogoutHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		s.sm.Delete(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// RegisterHandler ...
func (s *Server) RegisterHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			if s.config.OpenRegistrations {
				s.render("register", w, ctx)
			} else {
				message := s.config.RegisterMessage

				if message == "" {
					message = s.tr(ctx, "ErrorRegisterDisabled")
				}

				ctx.Error = true
				ctx.Message = message
				s.render("error", w, ctx)
			}

			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			trdata := map[string]interface{}{
				"Error": err.Error(),
			}
			ctx.Message = s.tr(ctx, "ErrorValidateUsername", trdata)
			s.render("error", w, ctx)
			return
		}

		if s.db.HasUser(username) || s.db.HasFeed(username) {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorHasUserOrFeed")
			s.render("error", w, ctx)
			return
		}

		p := filepath.Join(s.config.Data, feedsDir)
		if err := os.MkdirAll(p, 0755); err != nil {
			log.WithError(err).Error("error creating feeds directory")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fn := filepath.Join(p, username)
		if _, err := os.Stat(fn); err == nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUsernameExists")
			s.render("error", w, ctx)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hash, err := s.pm.CreatePassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHashString(email))

		user := NewUser()
		user.Username = username
		user.Password = hash
		user.Recovery = recoveryHash
		user.URL = URLForUser(s.config.BaseURL, username)
		user.CreatedAt = time.Now()

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Infof("user registered: %v", user)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// LookupHandler ...
func (s *Server) LookupHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		prefix := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("prefix")))

		feeds := s.db.SearchFeeds(prefix)

		user := ctx.User

		var following []string
		if len(prefix) > 0 {
			for nick := range user.Following {
				if strings.HasPrefix(strings.ToLower(nick), prefix) {
					following = append(following, nick)
				}
			}
		} else {
			following = append(following, StringKeys(user.Following)...)
		}

		var matches []string

		matches = append(matches, feeds...)
		matches = append(matches, following...)

		matches = UniqStrings(matches)

		data, err := json.Marshal(matches)
		if err != nil {
			log.WithError(err).Error("error serializing lookup response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}

// SettingsHandler ...
func (s *Server) SettingsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			ctx.Title = s.tr(ctx, "PageSettingsTitle")
			ctx.Bookmarklet = url.QueryEscape(fmt.Sprintf(bookmarkletTemplate, s.config.BaseURL))
			s.render("settings", w, ctx)
			return
		}

		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)
		defer r.Body.Close()

		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))
		tagline := strings.TrimSpace(r.FormValue("tagline"))
		password := r.FormValue("password")

		theme := r.FormValue("theme")
		displayDatesInTimezone := r.FormValue("displayDatesInTimezone")
		displayTimePreference := r.FormValue("displayTimePreference")
		isFollowersPubliclyVisible := r.FormValue("isFollowersPubliclyVisible") == "on"
		isFollowingPubliclyVisible := r.FormValue("isFollowingPubliclyVisible") == "on"
		isBookmarksPubliclyVisible := r.FormValue("isBookmarksPubliclyVisible") == "on"

		avatarFile, _, err := r.FormFile("avatar_file")
		if err != nil && err != http.ErrMissingFile {
			log.WithError(err).Error("error parsing form file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		if password != "" {
			hash, err := s.pm.CreatePassword(password)
			if err != nil {
				log.WithError(err).Error("error creating password hash")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			user.Password = hash
		}

		if avatarFile != nil {
			opts := &ImageOptions{
				Resize: true,
				Width:  AvatarResolution,
				Height: AvatarResolution,
			}
			_, err = StoreUploadedImage(
				s.config, avatarFile,
				avatarsDir, ctx.Username,
				opts,
			)
			if err != nil {
				ctx.Error = true
				ctx.Message = fmt.Sprintf("Error updating user: %s", err)
				s.render("error", w, ctx)
				return
			}
			avatarFn := filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.png", ctx.Username))
			if avatarHash, err := FastHashFile(avatarFn); err == nil {
				user.AvatarHash = avatarHash
			} else {
				log.WithError(err).Warnf("error updating avatar hash for %s", ctx.Username)
			}
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHashString(email))

		user.Recovery = recoveryHash
		user.Tagline = tagline

		user.Theme = theme
		user.DisplayDatesInTimezone = displayDatesInTimezone
		user.DisplayTimePreference = displayTimePreference
		user.IsFollowersPubliclyVisible = isFollowersPubliclyVisible
		user.IsFollowingPubliclyVisible = isFollowingPubliclyVisible
		user.IsBookmarksPubliclyVisible = isBookmarksPubliclyVisible

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUpdatingUser")
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgUpdateSettingsSuccess")
		s.render("error", w, ctx)
	}
}

// DeleteTokenHandler ...
func (s *Server) DeleteTokenHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		signature := p.ByName("signature")

		if err := s.db.DelToken(signature); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingToken")
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgDeleteTokenSuccess")

		http.Redirect(w, r, "/settings", http.StatusFound)

	}
}

// FollowersHandler ...
func (s *Server) FollowersHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingProfile")
				s.render("error", w, ctx)
				return
			}

			if !user.IsFollowersPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
				return
			}
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingProfile")
				s.render("error", w, ctx)
				return
			}
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserOrFeedNotFound")
			s.render("404", w, ctx)
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Followers); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		trdata := map[string]interface{}{
			"Username": nick,
		}
		ctx.Title = s.tr(ctx, "PageUserFollowersTitle", trdata)
		s.render("followers", w, ctx)
	}
}

// FollowingHandler ...
func (s *Server) FollowingHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingProfile")
				s.render("error", w, ctx)
				return
			}

			if !user.IsFollowingPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
				return
			}
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserNotFound")
			s.render("404", w, ctx)
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Followers); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		trdata := map[string]interface{}{
			"Username": nick,
		}
		ctx.Title = s.tr(ctx, "PageUserFollowingTitle", trdata)
		s.render("following", w, ctx)
	}
}

// ExternalHandler ...
func (s *Server) ExternalHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		uri := r.URL.Query().Get("uri")
		nick := r.URL.Query().Get("nick")

		if uri == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoExternalFeed")
			s.render("error", w, ctx)
			return
		}

		if nick == "" {
			log.Warn("no nick given to external profile request")
		}

		if !s.cache.IsCached(uri) {
			// TODO: Make this a Task?
			s.tasks.DispatchFunc(func() error {
				sources := make(types.Feeds)
				sources[types.Feed{Nick: nick, URL: uri}] = true
				s.cache.FetchTwts(s.config, s.archive, sources, nil)
				return nil
			})
		}

		twts := FilterTwts(ctx.User, s.cache.GetByURL(uri))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingTimeline")
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		if len(ctx.Twts) > 0 {
			ctx.Twter = ctx.Twts[0].Twter()
		} else {
			ctx.Twter = types.Twter{Nick: nick, URL: uri}
		}

		if ctx.Twter.Avatar == "" {
			avatar := GetExternalAvatar(s.config, ctx.Twter)
			if avatar != "" {
				ctx.Twter.Avatar = URLForExternalAvatar(s.config, uri)
			}
		}

		// If no &nick= provided try to guess a suitable nick
		// from the feed or some heuristics from the feed's URI
		// (borrowed from Yarns)
		if nick == "" {
			if ctx.Twter.Nick != "" {
				nick = ctx.Twter.Nick
			} else {
				// TODO: Move this logic into types/lextwt and types/retwt
				if u, err := url.Parse(uri); err == nil {
					if strings.HasSuffix(u.Path, "/twtxt.txt") {
						if rest := strings.TrimSuffix(u.Path, "/twtxt.txt"); rest != "" {
							nick = strings.Trim(rest, "/")
						} else {
							nick = u.Hostname()
						}
					} else if strings.HasSuffix(u.Path, ".txt") {
						base := filepath.Base(u.Path)
						if name := strings.TrimSuffix(base, filepath.Ext(base)); name != "" {
							nick = name
						} else {
							nick = u.Hostname()
						}
					} else {
						nick = Slugify(uri)
					}
				}
			}
		}

		following := make(map[string]string)
		for followingNick, followingTwter := range ctx.Twter.Follow {
			following[followingNick] = followingTwter.URL
		}

		ctx.Profile = types.Profile{
			Type: "External",

			Username: nick,
			Tagline:  ctx.Twter.Tagline,
			Avatar:   URLForExternalAvatar(s.config, uri),
			URL:      uri,

			Following:  following,
			NFollowing: ctx.Twter.Following,
			NFollowers: ctx.Twter.Followers,

			ShowFollowing: true,
			ShowFollowers: true,

			Follows:    ctx.User.Follows(uri),
			FollowedBy: ctx.User.FollowedBy(uri),
			Muted:      ctx.User.HasMuted(uri),
		}

		if len(twts) > 0 {
			ctx.Profile.LastPostedAt = twts[0].Created()
		}

		trdata := map[string]interface{}{}
		trdata["Nick"] = nick
		trdata["URL"] = uri
		ctx.Title = s.tr(ctx, "PageExternalProfileTitle", trdata)
		s.render("externalProfile", w, ctx)
	}
}

// ExternalFollowingHandler ...
func (s *Server) ExternalFollowingHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		uri := r.URL.Query().Get("uri")
		nick := r.URL.Query().Get("nick")

		if uri == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoExternalFeed")
			s.render("error", w, ctx)
			return
		}

		if nick == "" {
			log.Warn("no nick given to external profile request")
		}

		if !s.cache.IsCached(uri) {
			// TODO: Make this a Task?
			s.tasks.DispatchFunc(func() error {
				sources := make(types.Feeds)
				sources[types.Feed{Nick: nick, URL: uri}] = true
				s.cache.FetchTwts(s.config, s.archive, sources, nil)
				return nil
			})
		}

		twts := s.cache.GetByURL(uri)

		if len(twts) > 0 {
			ctx.Twter = twts[0].Twter()
		} else {
			ctx.Twter = types.Twter{Nick: nick, URL: uri}
		}

		if ctx.Twter.Avatar == "" {
			avatar := GetExternalAvatar(s.config, ctx.Twter)
			if avatar != "" {
				ctx.Twter.Avatar = URLForExternalAvatar(s.config, uri)
			}
		}

		// If no &nick= provided try to guess a suitable nick
		// from the feed or some heuristics from the feed's URI
		// (borrowed from Yarns)
		if nick == "" {
			if ctx.Twter.Nick != "" {
				nick = ctx.Twter.Nick
			} else {
				// TODO: Move this logic into types/lextwt and types/retwt
				if u, err := url.Parse(uri); err == nil {
					if strings.HasSuffix(u.Path, "/twtxt.txt") {
						if rest := strings.TrimSuffix(u.Path, "/twtxt.txt"); rest != "" {
							nick = strings.Trim(rest, "/")
						} else {
							nick = u.Hostname()
						}
					} else if strings.HasSuffix(u.Path, ".txt") {
						base := filepath.Base(u.Path)
						if name := strings.TrimSuffix(base, filepath.Ext(base)); name != "" {
							nick = name
						} else {
							nick = u.Hostname()
						}
					} else {
						nick = Slugify(uri)
					}
				}
			}
		}

		following := make(map[string]string)
		for followingNick, followingTwter := range ctx.Twter.Follow {
			following[followingNick] = followingTwter.URL
		}

		ctx.Profile = types.Profile{
			Type: "External",

			Username: nick,
			Tagline:  ctx.Twter.Tagline,
			Avatar:   URLForExternalAvatar(s.config, uri),
			URL:      uri,

			Following:  following,
			NFollowing: ctx.Twter.Following,
			NFollowers: ctx.Twter.Followers,

			ShowFollowing: true,
			ShowFollowers: true,

			Follows:    ctx.User.Follows(uri),
			FollowedBy: ctx.User.FollowedBy(uri),
			Muted:      ctx.User.HasMuted(uri),
		}

		if len(twts) > 0 {
			ctx.Profile.LastPostedAt = twts[0].Created()
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Following); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		trdata := map[string]interface{}{}
		trdata["Nick"] = nick
		trdata["URL"] = uri
		ctx.Title = s.tr(ctx, "PageExternalFollowingTitle", trdata)
		s.render("externalFollowing", w, ctx)
	}
}

// ExternalAvatarHandler ...
func (s *Server) ExternalAvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		uri := r.URL.Query().Get("uri")

		if uri == "" {
			log.Warn("no uri provided for external avatar")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		slug := Slugify(uri)

		fn := filepath.Join(s.config.Data, externalDir, fmt.Sprintf("%s.png", slug))
		w.Header().Set("Content-Type", "image/png")

		if !FileExists(fn) {
			log.Warnf("no external avatar found for %s", slug)
			http.Error(w, "External avatar not found", http.StatusNotFound)
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("os.Stat() error")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))

		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Etag", etag)
		if r.Method == http.MethodHead {
			return
		}

		if r.Method == http.MethodHead {
			return
		}

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening avatar file")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(w, f); err != nil {
			log.WithError(err).Error("error writing avatar response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// ResetPasswordHandler ...
func (s *Server) ResetPasswordHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			ctx.Title = s.tr(ctx, "PageResetPasswordTitle")
			s.render("resetPassword", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		email := strings.TrimSpace(r.FormValue("email"))
		recovery := fmt.Sprintf("email:%s", FastHashString(email))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		// Check if user exist
		if !s.db.HasUser(username) {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserNotFound")
			s.render("error", w, ctx)
			return
		}

		// Get user object from DB
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorGetUser")
			s.render("error", w, ctx)
			return
		}

		if recovery != user.Recovery {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserRecovery")
			s.render("error", w, ctx)
			return
		}

		// Create magic link expiry time
		now := time.Now()
		secs := now.Unix()
		expiresAfterSeconds := int64(600) // Link expires after 10 minutes

		expiryTime := secs + expiresAfterSeconds

		// Create magic link
		token := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			jwt.MapClaims{"username": username, "expiresAt": expiryTime},
		)
		tokenString, err := token.SignedString([]byte(s.config.MagicLinkSecret))
		if err != nil {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		if err := SendPasswordResetEmail(s.config, user, email, tokenString); err != nil {
			log.WithError(err).Errorf("unable to send reset password email to %s", user.Username)
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		log.Infof("reset password email sent for %s", user.Username)

		// Show success msg
		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgUserRecoveryRequestSent")
		s.render("error", w, ctx)
	}
}

// ResetPasswordMagicLinkHandler ...
func (s *Server) ResetPasswordMagicLinkHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		// Get token from query string
		tokens, ok := r.URL.Query()["token"]

		// Check if valid token
		if !ok || len(tokens[0]) < 1 {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidToken")
			s.render("error", w, ctx)
			return
		}

		tokenEmail := tokens[0]
		ctx.PasswordResetToken = tokenEmail

		// Show newPassword page
		s.render("newPassword", w, ctx)
	}
}

// NewPasswordHandler ...
func (s *Server) NewPasswordHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			return
		}

		password := r.FormValue("password")
		tokenEmail := r.FormValue("token")

		// Check if token is valid
		token, err := jwt.Parse(tokenEmail, func(token *jwt.Token) (interface{}, error) {

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(s.config.MagicLinkSecret), nil
		})

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

			var username = fmt.Sprintf("%v", claims["username"])
			var expiresAt int = int(claims["expiresAt"].(float64))

			now := time.Now()
			secs := now.Unix()

			// Check token expiry
			if secs > int64(expiresAt) {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorTokenExpires")
				s.render("error", w, ctx)
				return
			}

			user, err := s.db.GetUser(username)
			if err != nil {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorGetUser")
				s.render("error", w, ctx)
				return
			}

			// Reset password
			if password != "" {
				hash, err := s.pm.CreatePassword(password)
				if err != nil {
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorGetUser")
					s.render("error", w, ctx)
					return
				}

				user.Password = hash

				// Save user
				if err := s.db.SetUser(username, user); err != nil {
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorGetUser")
					s.render("error", w, ctx)
					return
				}
			}

			log.Infof("password changed: %v", user)

			// Show success msg
			ctx.Error = false
			ctx.Message = s.tr(ctx, "MsgPasswordResetSuccess")
			s.render("error", w, ctx)
		} else {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}
	}
}

// TaskHandler ...
func (s *Server) TaskHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		uuid := p.ByName("uuid")

		if uuid == "" {
			log.Warn("no task uuid provided")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		t, ok := s.tasks.Lookup(uuid)
		if !ok {
			log.Warnf("no task found by uuid: %s", uuid)
			http.Error(w, "Task Not Found", http.StatusNotFound)
			return
		}

		data, err := json.Marshal(t.Result())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)

	}
}

// SyndicationHandler ...
func (s *Server) SyndicationHandler() httprouter.Handle {
	formatTwt := FormatTwtFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var (
			twts    types.Twts
			profile types.Profile
			err     error
		)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick != "" {
			if s.db.HasUser(nick) {
				if user, err := s.db.GetUser(nick); err == nil {
					profile = user.Profile(s.config.BaseURL, nil)
					twts = s.cache.GetByURL(profile.URL)
				} else {
					log.WithError(err).Error("error loading user object")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else if s.db.HasFeed(nick) {
				if feed, err := s.db.GetFeed(nick); err == nil {
					profile = feed.Profile(s.config.BaseURL, nil)
					twts = s.cache.GetByURL(profile.URL)
				} else {
					log.WithError(err).Error("error loading user object")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}
		} else {
			twts = s.cache.GetByView(localViewKey)

			profile = types.Profile{
				Type:     "Local",
				Username: s.config.Name,
				Tagline:  "", // TODO: Maybe Twtxt Pods should have a configurable description?
				URL:      s.config.BaseURL,
			}
		}

		if err != nil {
			log.WithError(err).Errorf("errorloading feeds for %s", nick)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			w.Header().Set(
				"Last-Modified",
				twts[len(twts)].Created().Format(http.TimeFormat),
			)
			return
		}

		now := time.Now()
		// feed author.email
		email := ""
		if nick == "" {
			email = s.config.AdminEmail
		}
		// main feed
		feed := &feeds.Feed{
			Title:       fmt.Sprintf("%s Twtxt Atom Feed", profile.Username),
			Link:        &feeds.Link{Href: profile.URL},
			Description: profile.Tagline,
			Author:      &feeds.Author{Name: profile.Username, Email: email},
			Created:     now,
		}
		// feed items
		var items []*feeds.Item

		for _, twt := range twts {
			url := URLForTwt(s.config.BaseURL, twt.Hash())
			items = append(items, &feeds.Item{
				Id:          url,
				Title:       string(formatTwt(twt)),
				Link:        &feeds.Link{Href: url},
				Author:      &feeds.Author{Name: twt.Twter().Nick},
				Description: string(formatTwt(twt)),
				Created:     twt.Created(),
			},
			)
		}
		feed.Items = items

		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		data, err := feed.ToAtom()
		if err != nil {
			log.WithError(err).Error("error serializing feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write([]byte(data))
	}
}

// PodVersionHandler ...
func (s *Server) PodVersionHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if r.Header.Get("Accept") == "application/json" {
			data, err := json.Marshal(s.config.Version)
			if err != nil {
				log.WithError(err).Error("error serializing pod version response")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		} else {
			ctx := NewContext(s, r)
			s.render("version", w, ctx)
		}
	}
}

// PodConfigHandler ...
func (s *Server) PodConfigHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		data, err := json.Marshal(s.config.Settings())
		if err != nil {
			log.WithError(err).Error("error serializing pod config response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}

// TransferFeedHandler...
func (s *Server) TransferFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		feedName := NormalizeFeedName(p.ByName("name"))
		transferToName := NormalizeFeedName(p.ByName("transferTo"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoFeed")
			s.render("error", w, ctx)
			return
		}

		if transferToName == "" {
			// Get feed followers list
			if s.db.HasFeed(feedName) {
				feed, err := s.db.GetFeed(feedName)
				if err != nil {
					log.WithError(err).Errorf("Error loading feed object for %s", feedName)
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorGetFeed")
					s.render("error", w, ctx)
					return
				}

				ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
				s.render("transferFeed", w, ctx)
				return
			}
		}

		// Get feed
		if s.db.HasFeed(feedName) {

			feed, err := s.db.GetFeed(feedName)
			if err != nil {
				log.WithError(err).Errorf("Error loading feed object for %s", feedName)
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorGetFeed")
				s.render("error", w, ctx)
				return
			}

			// Get FromUser
			fromUser, err := s.db.GetUser(ctx.User.Username)
			if err != nil {
				log.WithError(err).Errorf("Error loading user")
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorGetFeed")
				s.render("error", w, ctx)
				return
			}

			// Get ToUser
			toUser, err := s.db.GetUser(transferToName)
			if err != nil {
				log.WithError(err).Errorf("Error loading user")
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorGetUser")
				s.render("error", w, ctx)
				return
			}

			// Transfer ownerships
			_ = RemoveFeedOwnership(s.db, fromUser, feed)
			_ = AddFeedOwnership(s.db, toUser, feed)

			ctx.Error = false
			ctx.Message = s.tr(ctx, "MsgTransferFeedSuccess")
			s.render("error", w, ctx)
		}
	}
}

// DeleteAllHandler ...
func (s *Server) DeleteAllHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		// Get all user feeds
		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
			s.render("error", w, ctx)
			return
		}

		for _, feed := range feeds {
			// Get user's owned feeds
			if ctx.User.OwnsFeed(feed.Name) {
				// Get twts in a feed
				nick := feed.Name
				if nick != "" {
					if s.db.HasFeed(nick) {
						// Fetch feed twts
						twts, err := GetAllTwts(s.config, nick)
						if err != nil {
							ctx.Error = true
							ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
							s.render("error", w, ctx)
							return
						}

						// Parse twts to search and remove uploaded media
						for _, twt := range twts {
							// Delete archived twts
							if err := s.archive.Del(twt.Hash()); err != nil {
								ctx.Error = true
								ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
								s.render("error", w, ctx)
								return
							}

							mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

							// Remove all uploaded media in a twt
							for _, mediaPath := range mediaPaths {
								// Delete .png
								fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
										s.render("error", w, ctx)
										return
									}
								}
							}
						}
					}
				}

				// Delete feed
				if err := s.db.DelFeed(nick); err != nil {
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
					s.render("error", w, ctx)
					return
				}

				// Delete feeds's twtxt.txt
				fn := filepath.Join(s.config.Data, feedsDir, nick)
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing feed")
						ctx.Error = true
						ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
						s.render("error", w, ctx)
					}
				}

				// Delete feed from cache
				s.cache.DeleteFeeds(feed.Source())
			}
		}

		// Get user's primary feed twts
		twts, err := GetAllTwts(s.config, ctx.User.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
			s.render("error", w, ctx)
			return
		}

		// Parse twts to search and remove primary feed uploaded media
		for _, twt := range twts {
			// Delete archived twts
			if err := s.archive.Del(twt.Hash()); err != nil {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
				s.render("error", w, ctx)
				return
			}

			mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

			// Remove all uploaded media in a twt
			for _, mediaPath := range mediaPaths {
				// Delete .png
				fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
						s.render("error", w, ctx)
					}
				}
			}
		}

		// Delete user's primary feed
		if err := s.db.DelFeed(ctx.User.Username); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
			s.render("error", w, ctx)
			return
		}

		// Delete user's twtxt.txt
		fn := filepath.Join(s.config.Data, feedsDir, ctx.User.Username)
		if FileExists(fn) {
			if err := os.Remove(fn); err != nil {
				log.WithError(err).Error("error removing user's feed")
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
				s.render("error", w, ctx)
			}
		}

		// Delete user
		if err := s.db.DelUser(ctx.Username); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingAccount")
			s.render("error", w, ctx)
			return
		}

		// Delete user's feed from cache
		s.cache.DeleteFeeds(ctx.User.Source())

		s.sm.Delete(w, r)
		ctx.Authenticated = false

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgDeleteAccountSuccess")
		s.render("error", w, ctx)
	}
}

// DeleteAccountHandler ...
func (s *Server) DeleteAccountHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		user := ctx.User

		allFeeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingFeeds")
			s.render("error", w, ctx)
			return
		}

		var userFeeds []*Feed

		for _, feed := range allFeeds {
			if user.OwnsFeed(feed.Name) {
				userFeeds = append(userFeeds, feed)
			}
		}

		ctx.UserFeeds = userFeeds
		s.render("deleteAccount", w, ctx)
	}
}
