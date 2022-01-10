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
	"github.com/renstrom/shortuuid"
	log "github.com/sirupsen/logrus"
)

// ManagePodHandler ...
func (s *Server) ManagePodHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		if r.Method == "GET" {
			s.render("managePod", w, ctx)
			return
		}

		name := strings.TrimSpace(r.FormValue("podName"))
		logo := strings.TrimSpace(r.FormValue("podLogo"))
		description := strings.TrimSpace(r.FormValue("podDescription"))
		maxTwtLength := SafeParseInt(r.FormValue("maxTwtLength"), s.config.MaxTwtLength)
		avatarResolution := SafeParseInt(r.FormValue("avatarResolution"), s.config.AvatarResolution)
		mediaResolution := SafeParseInt(r.FormValue("mediaResolution"), s.config.MediaResolution)
		openProfiles := r.FormValue("enableOpenProfiles") == "on"
		openRegistrations := r.FormValue("enableOpenRegistrations") == "on"
		whitelistedImages := r.FormValue("whitelistedImages")
		blacklistedFeeds := r.FormValue("blacklistedFeeds")
		enabledFeatures := r.FormValue("enabledFeatures")

		displayDatesInTimezone := r.FormValue("displayDatesInTimezone")
		displayTimePreference := r.FormValue("displayTimePreference")
		openLinksInPreference := r.FormValue("openLinksInPreference")
		displayImagesPreference := r.FormValue("displayImagesPreference")
		displayMedia := r.FormValue("displayMedia") == "on"
		originalMedia := r.FormValue("originalMedia") == "on"

		// Clean lines from DOS (\r\n) to UNIX (\n)
		logo = strings.ReplaceAll(logo, "\r\n", "\n")

		whitelistedImages = strings.Trim(strings.ReplaceAll(whitelistedImages, "\r\n", "\n"), "\n")
		blacklistedFeeds = strings.Trim(strings.ReplaceAll(blacklistedFeeds, "\r\n", "\n"), "\n")
		enabledFeatures = strings.Trim(strings.ReplaceAll(enabledFeatures, "\r\n", "\n"), "\n")

		// Update pod name
		if name != "" {
			s.config.Name = name
		} else {
			ctx.Error = true
			ctx.Message = "Pod name not specified"
			s.render("error", w, ctx)
			return
		}

		// Update pod logo
		if logo != "" {
			s.config.Logo = logo
		} else {
			ctx.Error = true
			ctx.Message = "Pod logo not provided"
			s.render("error", w, ctx)
			return
		}

		// Update pod description
		if description != "" {
			s.config.Description = description
		} else {
			ctx.Error = true
			ctx.Message = "Pod description not provided"
			s.render("error", w, ctx)
			return
		}

		// Update Max Twt Length
		s.config.MaxTwtLength = maxTwtLength
		// Update Avatar Resolution
		s.config.AvatarResolution = avatarResolution
		// Update Media Resolution
		s.config.MediaResolution = mediaResolution
		// Update open profiles
		s.config.OpenProfiles = openProfiles
		// Update open registrations
		s.config.OpenRegistrations = openRegistrations

		// Update WhitelistedImages
		if err := WithWhitelistedImages(strings.Split(whitelistedImages, "\n"))(s.config); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error applying whitelist for images: %s", err)
			s.render("error", w, ctx)
			return
		}

		// Update BlacklistedFeeds
		if err := WithBlacklistedFeeds(strings.Split(blacklistedFeeds, "\n"))(s.config); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error applying blacklist for feeds: %s", err)
			s.render("error", w, ctx)
			return
		}

		// Update Enabled Optional Features

		features, err := FeaturesFromStrings(strings.Split(enabledFeatures, "\n"))
		if err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error extracting features: %s", err)
			s.render("error", w, ctx)
			return
		}
		if err := WithEnabledFeatures(features)(s.config); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error applying features: %s", err)
			s.render("error", w, ctx)
			return
		}

		// Update Pod Settings (overrideable by Users)
		s.config.DisplayDatesInTimezone = displayDatesInTimezone
		s.config.DisplayTimePreference = displayTimePreference
		s.config.OpenLinksInPreference = openLinksInPreference
		s.config.DisplayImagesPreference = displayImagesPreference
		s.config.DisplayMedia = displayMedia
		s.config.OriginalMedia = originalMedia

		// Save config file
		if err := s.config.Settings().Save(filepath.Join(s.config.Data, "settings.yaml")); err != nil {
			log.WithError(err).Error("error saving config")
			ctx.Error = true
			ctx.Message = "Error saving pod settings"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Pod updated successfully"
		s.render("error", w, ctx)
	}
}

// ManageUsersHandler ...
func (s *Server) ManageUsersHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		s.render("manageUsers", w, ctx)
	}
}

// AddUserHandler ...
func (s *Server) AddUserHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))

		// Random password -- User is expected to user "Password Reset"
		password := shortuuid.New()

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if s.db.HasUser(username) || s.db.HasFeed(username) {
			ctx.Error = true
			ctx.Message = "User or Feed with that name already exists! Please pick another!"
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
			ctx.Message = "Deleted user with that username already exists! Please pick another!"
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
		user.Recovery = recoveryHash
		user.Password = hash
		user.URL = URLForUser(s.config.BaseURL, username)
		user.CreatedAt = time.Now()

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx.Error = false
		ctx.Message = "User successfully created"
		s.render("error", w, ctx)
	}
}

// DelUserHandler ...
func (s *Server) DelUserHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))

		user, err := s.db.GetUser(username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", username)
			ctx.Error = true
			ctx.Message = "Error deleting account"
			s.render("error", w, ctx)
			return
		}

		// Get all user feeds
		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		for _, feed := range feeds {
			// Get user's owned feeds
			if user.OwnsFeed(feed.Name) {
				// Get twts in a feed
				nick := feed.Name
				if nick != "" {
					if s.db.HasFeed(nick) {
						// Fetch feed twts
						twts, err := GetAllTwts(s.config, nick)
						if err != nil {
							ctx.Error = true
							ctx.Message = "An error occured whilst deleting your account"
							s.render("error", w, ctx)
							return
						}

						// Parse twts to search and remove uploaded media
						for _, twt := range twts {
							// Delete archived twts
							if err := s.archive.Del(twt.Hash()); err != nil {
								ctx.Error = true
								ctx.Message = "An error occured whilst deleting your account"
								s.render("error", w, ctx)
								return
							}

							mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

							// Remove all uploaded media in a twt
							for _, mediaPath := range mediaPaths {
								// Delete .png
								fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = "An error occured whilst deleting your account"
										s.render("error", w, ctx)
										return
									}
								}
							}
						}
					}
				}

				// Delete feed
				if err := s.db.DelFeed(nick); err != nil {
					ctx.Error = true
					ctx.Message = "An error occured whilst deleting your account"
					s.render("error", w, ctx)
					return
				}

				// Delete feeds's twtxt.txt
				fn := filepath.Join(s.config.Data, feedsDir, nick)
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing feed")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}

				// Delete feed from cache
				s.cache.DeleteFeeds(feed.Source())
			}
		}

		// Get user's primary feed twts
		twts, err := GetAllTwts(s.config, user.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Parse twts to search and remove primary feed uploaded media
		for _, twt := range twts {
			// Delete archived twts
			if err := s.archive.Del(twt.Hash()); err != nil {
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
				return
			}

			mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

			// Remove all uploaded media in a twt
			for _, mediaPath := range mediaPaths {
				// Delete .png
				fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}
			}
		}

		// Delete user's primary feed
		if err := s.db.DelFeed(user.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's twtxt.txt
		fn := filepath.Join(s.config.Data, feedsDir, user.Username)
		if FileExists(fn) {
			if err := os.Remove(fn); err != nil {
				log.WithError(err).Error("error removing user's feed")
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
			}
		}

		// Delete user
		if err := s.db.DelUser(user.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's feed from cache
		s.cache.DeleteFeeds(user.Source())

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
	}
}

// DelFeedHandler ...
func (s *Server) DelFeedHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		name := NormalizeFeedName(r.FormValue("name"))

		feed, err := s.db.GetFeed(name)
		if err != nil {
			log.WithError(err).Errorf("error loading feed object for %s", name)
			ctx.Error = true
			ctx.Message = "Error deleting feed"
			s.render("error", w, ctx)
			return
		}

		// Delete feed
		if err := s.db.DelFeed(feed.Name); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting the feed"
			s.render("error", w, ctx)
			return
		}

		// Delete feeds's twtxt.txt
		fn := filepath.Join(s.config.Data, feedsDir, name)
		if FileExists(fn) {
			if err := os.Remove(fn); err != nil {
				log.WithError(err).Error("error removing feed")
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting the feed"
				s.render("error", w, ctx)
			}
		}

		// Delete feed from cache
		s.cache.DeleteFeeds(feed.Source())

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
	}
}

// RstUserHandler ...
func (s *Server) RstUserHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))

		trdata := map[string]interface{}{}
		trdata["Nick"] = username

		user, err := s.db.GetUser(username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", username)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorGetUser")
			s.render("error", w, ctx)
			return
		}

		newPassword := GenerateRandomToken()

		hash, err := s.pm.CreatePassword(newPassword)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidPassword")
			s.render("error", w, ctx)
			return
		}

		user.Password = hash

		// Save user
		if err := s.db.SetUser(username, user); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorSetUser", trdata)
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf(
			"Successfully reset password for %s to: %s",
			username, newPassword,
		)
		s.render("error", w, ctx)
	}
}

// RefreshCacheHandler ...
func (s *Server) RefreshCacheHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	var UpdateFeeds Job
	for _, entry := range s.cron.Entries() {
		if entry.Job.(Job).String() == "UpdateFeeds" {
			UpdateFeeds = entry.Job.(Job)
			break
		}
	}
	if UpdateFeeds == nil {
		log.Fatal("UpdateFeeds job not found in cron")
	}

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		s.tasks.DispatchFunc(func() error {
			s.cache.Reset()
			UpdateFeeds.Run()
			return nil
		})

		ctx.Error = false
		ctx.Message = "Successfully deleted cache and started fetch cycle"
		s.render("error", w, ctx)
	}
}

// ManagePeersHandler ...
func (s *Server) ManagePeersHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		ctx.Peers = s.cache.GetPeers()

		s.render("managePeers", w, ctx)
	}
}

// ManageJobsHandler ...
func (s *Server) ManageJobsHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		if r.Method == http.MethodPost {
			name := strings.TrimSpace(r.FormValue("name"))

			var job Job
			for _, entry := range s.cron.Entries() {
				if strings.EqualFold(name, entry.Job.(Job).String()) {
					job = entry.Job.(Job)
					break
				}
			}

			if job == nil {
				ctx.Error = true
				ctx.Message = fmt.Sprintf("No job found by that name: %s", name)
				s.render("404", w, ctx)
				return
			}

			s.tasks.DispatchFunc(func() error {
				job.Run()
				return nil
			})

			ctx.Error = false
			ctx.Message = fmt.Sprintf("Job %s successfully queued for execution", name)
			s.render("error", w, ctx)

			return
		}

		ctx.Jobs = s.cron.Entries()
		s.render("manageJobs", w, ctx)
	}
}
