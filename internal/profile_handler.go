package internal

import (
	"fmt"
	"net/http"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

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

		profile.FollowedBy = s.cache.FollowedBy(ctx.User, profile.URI)

		ctx.Profile = profile

		ctx.Links = append(ctx.Links, Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(profile.URI)),
			Rel:  "webmention",
		})

		ctx.Alternatives = append(ctx.Alternatives, Alternatives{
			Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Nick),
				URL:   profile.URI,
			},
			Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Nick),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URI)),
			},
		}...)

		twts := FilterTwts(ctx.User, s.cache.GetByURL(profile.URI))

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

		ctx.Title = fmt.Sprintf("%s's Profile: %s", profile.Nick, profile.Description)
		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		s.render("profile", w, ctx)
	}
}
