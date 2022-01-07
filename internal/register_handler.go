package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// RegisterHandler ...
func (s *Server) RegisterHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			if s.config.OpenRegistrations {
				s.render("register", w, ctx)
			} else {
				message := s.config.RegisterMessage

				if message == "" {
					message = s.tr(ctx, "ErrorRegisterDisabled")
				}

				ctx.Error = true
				ctx.Message = message
				s.render("error", w, ctx)
			}

			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			trdata := map[string]interface{}{
				"Error": err.Error(),
			}
			ctx.Message = s.tr(ctx, "ErrorValidateUsername", trdata)
			s.render("error", w, ctx)
			return
		}

		if s.db.HasUser(username) || s.db.HasFeed(username) {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorHasUserOrFeed")
			s.render("error", w, ctx)
			return
		}

		p := filepath.Join(s.config.Data, feedsDir)
		if err := os.MkdirAll(p, 0755); err != nil {
			log.WithError(err).Error("error creating feeds directory")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fn := filepath.Join(p, username)
		if _, err := os.Stat(fn); err == nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUsernameExists")
			s.render("error", w, ctx)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hash, err := s.pm.CreatePassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHashString(email))

		user := NewUser()
		user.Username = username
		user.Password = hash
		user.Recovery = recoveryHash
		user.URL = URLForUser(s.config.BaseURL, username)
		user.CreatedAt = time.Now()

		// Default Feeds
		user.Follow(newsSpecialUser, s.config.URLForUser(newsSpecialUser)+"/twtxt.txt")
		user.Follow(supportSpecialUser, s.config.URLForUser(supportSpecialUser)+"/twtxt.txt")
		user.Follow(helpSpecialUser, s.config.URLForUser(helpSpecialUser)+"/twtxt.txt")

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Onboarding: Welcome new User and notify Poderator

		// TODO: Make this async?
		if s.config.Features.IsEnabled(FeatureInternalEvents) {
			if admin, err := s.db.GetUser(s.config.AdminUser); err == nil && !isAdminUser(user) {
				if twtxt, err := s.db.GetFeed(twtxtBot); err == nil {
					// TODO: Make this configurable?
					welcomeUserText := fmt.Sprintf(
						"👋 Hey @<%s %s/twtxt.txt>, welcome to %s, a [Yarn.social](https://yarn.social) Pod! To get started you may want to check out the pod's [Discover](/discover) feed. To follow a new feed or user check out [Feeds](/feeds) and [Follow](/follow). Once again, welcome! 🤗",
						user.Username, s.config.URLForUser(user.Username),
						s.config.Name,
					)
					newUserText := fmt.Sprintf(
						"👋 Hey @<%s %s/twtxt.txt>, a new user (@<%s %s/twtxt.txt>) has joined your pod %s! 🥳",
						admin.Username, s.config.URLForUser(admin.Username),
						user.Username, s.config.URLForUser(user.Username),
						s.config.Name,
					)
					s.cache.AddEvent(user, twtxt, welcomeUserText)
					s.cache.AddEvent(admin, twtxt, newUserText)
				}
			}
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	}
}
