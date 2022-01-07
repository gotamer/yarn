package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// PostHandler ...
func (s *Server) PostHandler() httprouter.Handle {
	//isLocalURL := IsLocalURLFactory(s.config)

	appendTwt := AppendTwtFactory(s.config, s.db)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		postAs := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))

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
			s.cache.FetchFeeds(s.config, s.archive, ctx.User.Source(), nil)

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

		var (
			//sources types.Feeds
			twt     types.Twt = types.NilTwt
			feedURL string
		)

		switch postAs {
		case "", user.Username:
			//sources = user.Source()
			feedURL = s.config.URLForUser(user.Username)

			if hash != "" && lastTwt.Hash() == hash {
				twt, err = appendTwt(user, nil, text, lastTwt.Created())
			} else {
				twt, err = appendTwt(user, nil, text)
			}
		default:
			if user.OwnsFeed(postAs) {
				feed, feedErr := s.db.GetFeed(postAs)
				if feedErr != nil {
					log.WithError(err).Error("error loading feed object")
					ctx.Error = true
					ctx.Message = s.tr(ctx, "ErrorPostingTwt")
					s.render("error", w, ctx)
					return
				}
				//		sources = feed.Source()
				feedURL = s.config.URLForUser(postAs)

				if hash != "" && lastTwt.Hash() == hash {
					twt, err = appendTwt(user, feed, text, lastTwt.Created)
				} else {
					twt, err = appendTwt(user, feed, text)
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
		// XXX: This is too slow and expensive :/
		// s.cache.FetchFeeds(s.config, s.archive, sources, nil)
		s.cache.InjectFeed(feedURL, twt)

		// Force User Views to be recalculated
		s.cache.DeleteUserViews(ctx.User)

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
