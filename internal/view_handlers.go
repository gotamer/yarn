package internal

import (
	"net/http"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

func (s *Server) getTimelineTwts(user *User) types.Twts {
	return s.cache.GetByUser(user, false)
}

func (s *Server) getDiscoverTwts(user *User) types.Twts {
	return s.cache.GetByUserView(user, discoverViewKey, false)
}

func (s *Server) getMentionedTwts(user *User) types.Twts {
	return s.cache.GetMentions(user, false)
}

func (s *Server) timelineUpdatedAt(user *User) time.Time {
	twts := s.getTimelineTwts(user)

	if len(twts) > 0 {
		return twts[0].Created()
	}

	return time.Time{}
}

func (s *Server) discoverUpdatedAt(user *User) time.Time {
	twts := s.getDiscoverTwts(user)

	if len(twts) > 0 {
		return twts[0].Created()
	}

	return time.Time{}
}

func (s *Server) lastMentionedAt(user *User) time.Time {
	twts := s.getMentionedTwts(user)

	if len(twts) > 0 {
		return twts[0].Created()
	}

	return time.Time{}
}

// TimelineHandler ...
func (s *Server) TimelineHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		ctx := NewContext(s, r)
		// ctx translate
		// TODO it's bad, I have no idea. @venjiang
		ctx.Translate(s.translator)

		var twts types.Twts

		if !ctx.Authenticated {
			twts = s.getDiscoverTwts(ctx.User)
			ctx.Title = s.tr(ctx, "PageLocalTimelineTitle")
		} else {
			ctx.Title = s.tr(ctx, "PageUserTimelineTitle")
			twts = s.getTimelineTwts(ctx.User)
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

		if ctx.Authenticated {
			lastTwt, _, err := GetLastTwt(s.config, ctx.User)
			if err != nil {
				log.WithError(err).Error("error getting user last twt")
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorTimelineLoad")
				s.render("error", w, ctx)
				return
			}
			ctx.LastTwt = lastTwt
		}

		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		if len(ctx.Twts) > 0 {
			ctx.TimelineUpdatedAt = ctx.Twts[0].Created()
		}

		s.render("timeline", w, ctx)
	}
}

// DiscoverHandler ...
func (s *Server) DiscoverHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		twts := s.getDiscoverTwts(ctx.User)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingDiscover")
			s.render("error", w, ctx)
			return
		}

		if ctx.Authenticated {
			lastTwt, _, err := GetLastTwt(s.config, ctx.User)
			if err != nil {
				log.WithError(err).Error("error getting user last twt")
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorLoadingDiscover")
				s.render("error", w, ctx)
				return
			}
			ctx.LastTwt = lastTwt
		}

		ctx.Title = s.tr(ctx, "PageDiscoverTitle")
		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		if len(ctx.Twts) > 0 {
			ctx.DiscoverUpdatedAt = ctx.Twts[0].Created()
		}

		s.render("timeline", w, ctx)
	}
}

// MentionsHandler ...
func (s *Server) MentionsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		twts := s.getMentionedTwts(ctx.User)
		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingMentions")
			s.render("error", w, ctx)
			return
		}

		ctx.Title = s.tr(ctx, "PageMentionsTitle")
		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		if len(ctx.Twts) > 0 {
			ctx.LastMentionedAt = ctx.Twts[0].Created()
		}

		s.render("timeline", w, ctx)
	}
}
