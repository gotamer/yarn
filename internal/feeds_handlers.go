package internal

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

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

		s.cache.DeleteUserViews(ctx.User)

		ctx.Error = false
		trdata["Feed"] = name
		ctx.Message = s.tr(ctx, "MsgCreateFeedSuccess", trdata)
		s.render("error", w, ctx)
	}
}

// FeedsHandler ...
func (s *Server) FeedsHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)
	canManageFeed := func(feed string, u *User) bool {
		if u.OwnsFeed(feed) {
			return true
		}
		if IsSpecialFeed(feed) && isAdminUser(u) {
			return true
		}
		return false
	}

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

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
			if canManageFeed(feed.Name, ctx.User) {
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

// ManageFeedHandler...
func (s *Server) ManageFeedHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)
	canManageFeed := func(feed string, u *User) bool {
		if u.OwnsFeed(feed) {
			return true
		}
		if IsSpecialFeed(feed) && isAdminUser(u) {
			return true
		}
		return false
	}

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

		if !canManageFeed(feed.Name, ctx.User) {
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

// DeleteFeedHandler...
func (s *Server) DeleteFeedHandler() httprouter.Handle {
	canDeleteFeed := func(feed string, u *User) bool {
		if IsSpecialFeed(feed) {
			return false
		}
		return u.OwnsFeed(feed)
	}

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
			} else {
				ctx.Message = s.tr(ctx, "ErrorLoadingFeed")
				s.render("error", w, ctx)
			}
			return
		}

		if !canDeleteFeed(feed.Name, ctx.User) {
			ctx.Error = true
			s.render("401", w, ctx)
			return
		}

		if err := DeleteFeed(s.db, ctx.User, feed); err != nil {
			log.WithError(err).Warnf("Error deleting feed %s", feed)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorDeletingFeed")
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgDeleteFeedSuccess")
		s.render("error", w, ctx)
	}
}
