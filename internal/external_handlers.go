package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

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

		if !s.cache.IsCached(uri) {
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
			ctx.Profile.LastSeenAt = twts[0].Created()
		}

		trdata := map[string]interface{}{}
		trdata["Nick"] = nick
		trdata["URL"] = uri
		ctx.Title = s.tr(ctx, "PageExternalProfileTitle", trdata)
		s.render("profile", w, ctx)
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

		if !s.cache.IsCached(uri) {
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
		s.render("following", w, ctx)
	}
}

// ExternalAvatarHandler ...
func (s *Server) ExternalAvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		uri := r.URL.Query().Get("uri")

		if uri == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		slug := Slugify(uri)

		fn := filepath.Join(s.config.Data, externalDir, fmt.Sprintf("%s.png", slug))
		w.Header().Set("Content-Type", "image/png")

		if !FileExists(fn) {
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
