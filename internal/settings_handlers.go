package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// SettingsHandler ...
func (s *Server) SettingsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			profile := ctx.User.Profile(s.config.BaseURL, ctx.User)

			followers := s.cache.GetFollowers(profile)

			profile.Followers = followers
			profile.NFollowers = len(followers)

			profile.FollowedBy = s.cache.FollowedBy(ctx.User, profile.URI)

			ctx.Profile = profile

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
		userTheme := r.FormValue("userTheme")
		displayDatesInTimezone := r.FormValue("displayDatesInTimezone")
		displayTimePreference := r.FormValue("displayTimePreference")
		openLinksInPreference := r.FormValue("openLinksInPreference")
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
		user.userTheme = userTheme
		user.DisplayDatesInTimezone = displayDatesInTimezone
		user.DisplayTimePreference = displayTimePreference
		user.OpenLinksInPreference = openLinksInPreference
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
