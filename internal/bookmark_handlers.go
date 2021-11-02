package internal

import (
	"net/http"
	"sort"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// BookmarkHandler ...
func (s *Server) BookmarkHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		hash := p.ByName("hash")
		if hash == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var err error

		twt, ok := s.cache.Lookup(hash)
		if !ok {
			// If the twt is not in the cache look for it in the archive
			if s.archive.Has(hash) {
				twt, err = s.archive.Get(hash)
				if err != nil {
					ctx.Error = true
					ctx.Message = "Error loading twt from archive, please try again"
					s.render("error", w, ctx)
					return
				}
			}
		}

		if twt.IsZero() {
			ctx.Error = true
			ctx.Message = "No matching twt found!"
			s.render("404", w, ctx)
			return
		}

		ctx.User.Bookmark(twt.Hash())

		if err := s.db.SetUser(ctx.Username, ctx.User); err != nil {
			ctx.Error = true
			ctx.Message = "Error updating user"
			s.render("error", w, ctx)
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully updated bookmarks"
		s.render("error", w, ctx)
	}
}

// BookmarksHandler ...
func (s *Server) BookmarksHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))

		var twts types.Twts

		getTwts := func(hashes []string) (twts types.Twts) {
			for _, hash := range hashes {
				if twt, ok := s.cache.Lookup(hash); ok {
					twts = append(twts, twt)
				} else if s.archive.Has(hash) {
					if twt, err := s.archive.Get(hash); err == nil {
						twts = append(twts, twt)
					} else {
						log.WithError(err).Errorf("error loading twt %s from archive", hash)
					}
				}
			}
			return
		}

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}

			if !user.IsBookmarksPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
				return
			}
			twts = getTwts(StringKeys(user.Bookmarks))
			sort.Sort(twts)
		} else {
			ctx.Error = true
			ctx.Message = "User Not Found"
			s.render("404", w, ctx)
			return
		}

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorTimelineLoad")
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		s.render("timeline", w, ctx)
	}
}
