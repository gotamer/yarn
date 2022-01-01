package internal

import (
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	securejoin "github.com/cyphar/filepath-securejoin"
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
				s.cache.FetchFeeds(s.config, s.archive, sources, nil)
				return nil
			})
		}

		if twter := s.cache.GetTwter(uri); twter != nil {
			ctx.Twter = *twter
		} else {
			ctx.Twter = types.Twter{Nick: nick, URI: uri}
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

		var follows types.Follows
		for nick, twter := range ctx.Twter.Follow {
			follows = append(follows, types.Follow{Nick: nick, URI: twter.URI})
		}

		ctx.Profile = types.Profile{
			Type: "External",

			Nick:        ctx.Twter.Nick,
			Description: ctx.Twter.Tagline,
			Avatar:      ctx.Twter.Avatar,
			URI:         ctx.Twter.URI,

			Following:  follows,
			NFollowing: ctx.Twter.Following,
			NFollowers: ctx.Twter.Followers,

			ShowFollowing: true,
			ShowFollowers: true,

			Follows:    ctx.User.Follows(uri),
			FollowedBy: s.cache.FollowedBy(ctx.User, uri),
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
				s.cache.FetchFeeds(s.config, s.archive, sources, nil)
				return nil
			})
		}

		if twter := s.cache.GetTwter(uri); twter != nil {
			ctx.Twter = *twter
		} else {
			ctx.Twter = types.Twter{Nick: nick, URI: uri}
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

		var follows types.Follows
		for nick, twter := range ctx.Twter.Follow {
			follows = append(follows, types.Follow{Nick: nick, URI: twter.URI})
		}

		ctx.Profile = types.Profile{
			Type: "External",

			Nick:        nick,
			Description: ctx.Twter.Tagline,
			Avatar:      URLForExternalAvatar(s.config, uri),
			URI:         uri,

			Following:  follows,
			NFollowing: ctx.Twter.Following,
			NFollowers: ctx.Twter.Followers,

			ShowFollowing: true,
			ShowFollowers: true,

			Follows:    ctx.User.Follows(uri),
			FollowedBy: ctx.User.FollowedBy(uri),
			Muted:      ctx.User.HasMuted(uri),
		}

		twts := s.cache.GetByURL(uri)

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

		uri := NormalizeURL(r.URL.Query().Get("uri"))
		if uri == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		slug := Slugify(uri)
		fn, err := securejoin.SecureJoin(filepath.Join(s.config.Data, externalDir), fmt.Sprintf("%s.png", slug))
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if !FileExists(fn) {
			domainNick := slug

			if twter := s.cache.GetTwter(uri); twter != nil {
				domainNick = twter.DomainNick()
			}

			img, err := GenerateAvatar(s.config.Name, domainNick)
			if err != nil {
				log.WithError(err).Errorf("error generating external avatar for %s", uri)
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
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("os.Stat() error")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Etag", fmt.Sprintf("W/\"%s-%s\"", slug, fileInfo.ModTime().Format(time.RFC3339)))
		w.Header().Set("Last-Modified", fileInfo.ModTime().Format(http.TimeFormat))

		http.ServeFile(w, r, fn)
	}
}
