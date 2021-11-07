package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rickb777/accept"
	"github.com/securisec/go-keywords"
	log "github.com/sirupsen/logrus"

	"git.mills.io/yarnsocial/yarn/types"
)

const (
	maxPermalinkTitle = 144
)

// PermalinkHandler ...
func (s *Server) PermalinkHandler() httprouter.Handle {
	isLocal := IsLocalURLFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		hash := p.ByName("hash")
		if hash == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var err error

		twt, inCache := s.cache.Lookup(hash)
		if !inCache {
			// If the twt is not in the cache look for it in the archive
			if s.archive.Has(hash) {
				twt, err = s.archive.Get(hash)
				if err != nil {
					if accept.PreferredContentTypeLike(r.Header, "text/html") == "text/html" {
						ctx.Error = true
						ctx.Message = s.tr(ctx, "ErrorLoadingTwtFromArchive")
						s.render("error", w, ctx)
					} else {
						http.Error(w, "Error loading twt from archive", http.StatusInternalServerError)
					}
					return
				}
			}
		}

		if twt.IsZero() {
			if accept.PreferredContentTypeLike(r.Header, "text/html") == "text/html" {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorNoMatchingTwt")
				s.render("404", w, ctx)
			} else {
				http.Error(w, "No matching twt by that hash", http.StatusNotFound)
			}
			return
		}

		if accept.PreferredContentTypeLike(r.Header, "application/json") == "application/json" {
			data, err := json.Marshal(twt)
			if err != nil {
				log.WithError(err).Error("error serializing twt response")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Last-Modified", twt.Created().Format(http.TimeFormat))
			_, _ = w.Write(data)
			return
		}

		who := twt.Twter().DomainNick()

		var image string
		if isLocal(twt.Twter().URL) {
			image = URLForAvatar(s.config.BaseURL, twt.Twter().Nick, "")
		} else {
			image = URLForExternalAvatar(s.config, twt.Twter().URL)
		}

		when := twt.Created().Format(time.RFC3339)
		what := twt.FormatText(types.TextFmt, s.config)

		var ks []string
		if ks, err = keywords.Extract(what); err != nil {
			log.WithError(err).Warn("error extracting keywords")
		}

		for _, m := range twt.Mentions() {
			ks = append(ks, m.Twter().Nick)
		}
		var tags types.TagList = twt.Tags()
		ks = append(ks, tags.Tags()...)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Last-Modified", twt.Created().Format(http.TimeFormat))
		if strings.HasPrefix(twt.Twter().URL, s.config.BaseURL) {
			w.Header().Set(
				"Link",
				fmt.Sprintf(
					`<%s/user/%s/webmention>; rel="webmention"`,
					s.config.BaseURL, twt.Twter().Nick,
				),
			)
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		title := fmt.Sprintf("%s \"%s\"", who, TextWithEllipsis(what, maxPermalinkTitle))

		ctx.Title = title
		ctx.Meta = Meta{
			Title:       title,
			Description: what,
			UpdatedAt:   when,
			Author:      who,
			Image:       image,
			URL:         URLForTwt(s.config.BaseURL, hash),
			Keywords:    strings.Join(ks, ", "),
		}
		if strings.HasPrefix(twt.Twter().URL, s.config.BaseURL) {
			ctx.Links = append(ctx.Links, Link{
				Href: fmt.Sprintf("%s/webmention", UserURL(twt.Twter().URL)),
				Rel:  "webmention",
			})
			ctx.Alternatives = append(ctx.Alternatives, Alternatives{
				Alternative{
					Type:  "text/plain",
					Title: fmt.Sprintf("%s's Twtxt Feed", twt.Twter().Nick),
					URL:   twt.Twter().URL,
				},
				Alternative{
					Type:  "application/atom+xml",
					Title: fmt.Sprintf("%s's Atom Feed", twt.Twter().Nick),
					URL:   fmt.Sprintf("%s/atom.xml", UserURL(twt.Twter().URL)),
				},
			}...)
		}

		ctx.Twts = FilterTwts(ctx.User, types.Twts{twt})
		s.render("permalink", w, ctx)
	}
}
