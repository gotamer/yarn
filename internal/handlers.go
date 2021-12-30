package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/gorilla/feeds"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
	"gopkg.in/yaml.v2"

	"git.mills.io/yarnsocial/yarn/types"
)

const (
	MediaResolution  = 850 // 850px width (maintaining aspect ratio)
	AvatarResolution = 360 // 360px width (maintaining aspect ratio)
	AsyncTaskLimit   = 5

	bookmarkletTemplate = `(function(){window.location.href="%s/?title="+document.title+"&url="+document.URL;})();`
)

var (
	ErrFeedImposter = errors.New("error: imposter detected, you do not own this feed")
)

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

		followers := s.cache.GetFollowers(profile)

		profile.Followers = followers
		profile.NFollowers = len(followers)

		profile.FollowedBy = s.cache.FollowedBy(ctx.User, profile.URL)

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

		fn, err := securejoin.SecureJoin(filepath.Join(s.config.Data, avatarsDir), fmt.Sprintf("%s.png", nick))
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if fileInfo, err := os.Stat(fn); err == nil {
			w.Header().Set("Etag", fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339)))
			w.Header().Set("Last-Modified", fileInfo.ModTime().Format(http.TimeFormat))
			http.ServeFile(w, r, fn)
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

		img, err := GenerateAvatar(s.config.Name, nick)
		if err != nil {
			log.WithError(err).Errorf("error generating avatar for %s", nick)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			return
		}

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
	//isLocalURL := IsLocalURLFactory(s.config)
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

			// Delete user's own feed as it was edited
			s.cache.DeleteFeeds(ctx.User.Source())

			// Update user's own timeline with their own new post.
			s.cache.FetchTwts(s.config, s.archive, ctx.User.Source(), nil)

			// Re-populate/Warm cache for User
			s.cache.GetByUser(ctx.User, true)

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
			// Delete user's own feed as it was edited
			s.cache.DeleteFeeds(ctx.User.Source())
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
		//var twt types.Twt = types.NilTwt

		switch postas {
		case "", user.Username:
			sources = user.Source()

			if hash != "" && lastTwt.Hash() == hash {
				_, err = AppendTwt(s.config, s.db, user, text, lastTwt.Created())
			} else {
				_, err = AppendTwt(s.config, s.db, user, text)
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
					_, err = AppendSpecial(s.config, s.db, postas, text, lastTwt.Created)
				} else {
					_, err = AppendSpecial(s.config, s.db, postas, text)
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

		// Update user's own timeline with their own new post.
		s.cache.FetchTwts(s.config, s.archive, sources, nil)

		// Re-populate/Warm cache for User
		s.cache.GetByUser(ctx.User, true)

		// WebMentions ...
		// TODO: Use a queue here instead?
		// TODO: Fix Webmentions
		// TODO: https://git.mills.io/yarnsocial/yarn/issues/438
		// TODO: https://git.mills.io/yarnsocial/yarn/issues/515
		/*
			if _, err := s.tasks.Dispatch(NewFuncTask(func() error {
				for _, m := range twt.Mentions() {
					twter := m.Twter()
					if !isLocalURL(twter.RequestURI) {
						if err := WebMention(twter.RequestURI, URLForTwt(s.config.BaseURL, twt.Hash())); err != nil {
							log.WithError(err).Warnf("error sending webmention to %s", twter.RequestURI)
						}
					}
				}
				return nil
			})); err != nil {
				log.WithError(err).Warn("error submitting task for webmentions")
			}
		*/

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

// LookupHandler ...
func (s *Server) LookupHandler() httprouter.Handle {
	isLocalURL := IsLocalURLFactory(s.config)
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		prefix := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("prefix")))

		user := ctx.User

		matches := make([]struct {
			Nick   string
			Avatar string
		}, 0)
		if len(prefix) > 0 {
			for nick, url := range user.Following {
				if strings.HasPrefix(strings.ToLower(nick), prefix) {
					var avatar string
					if isLocalURL(url) {
						avatar = URLForAvatar(s.config.BaseURL, nick, "")
					} else {
						avatar = URLForExternalAvatar(s.config, url)
					}
					matches = append(matches, struct {
						Nick   string
						Avatar string
					}{nick, avatar})
				}
			}
		}

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

// FollowersHandler ...
func (s *Server) FollowersHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))

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

			if !user.IsFollowersPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
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

		followers := s.cache.GetFollowers(profile)
		profile.Followers = followers
		profile.NFollowers = len(followers)

		ctx.Profile = profile

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

// TaskHandler ...
func (s *Server) TaskHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		uuid := p.ByName("uuid")

		if uuid == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		t, ok := s.tasks.Lookup(uuid)
		if !ok {
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
				Tagline:  s.config.Description,
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
			if len(twts) > 0 {
				w.Header().Set(
					"Last-Modified",
					twts[(len(twts)-1)].Created().Format(http.TimeFormat),
				)
			}
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
			what := twt.FormatText(types.TextFmt, s.config)
			title := TextWithEllipsis(what, maxPermalinkTitle)
			items = append(items, &feeds.Item{
				Id:          url,
				Title:       title,
				Link:        &feeds.Link{Href: url},
				Author:      &feeds.Author{Name: twt.Twter().DomainNick()},
				Description: twt.FormatText(types.HTMLFmt, s.config),
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

// PodInfoHandler ...
func (s *Server) PodInfoHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if r.Header.Get("Accept") == "application/json" {
			data, err := json.Marshal(Peer{
				Name:            s.config.Name,
				Description:     s.config.Description,
				SoftwareVersion: s.config.Version.FullVersion,
			})
			if err != nil {
				log.WithError(err).Error("error serializing pod version response")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		} else {
			ctx := NewContext(s, r)
			s.render("info", w, ctx)
		}
	}
}

// PodConfigHandler ...
func (s *Server) PodConfigHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		data, err := json.Marshal(s.config)
		if err != nil {
			log.WithError(err).Error("error serializing pod config response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
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
