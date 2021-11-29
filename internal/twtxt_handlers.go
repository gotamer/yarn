package internal

import (
	"fmt"
	std_ioutil "io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/badgerodon/ioutil"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const defaultPreambleTemplate = `# Twtxt is an open, distributed microblogging platform that
# uses human-readable text files, common transport protocols,
# and free software.
#
# Learn more about twtxt at  https://github.com/buckket/twtxt
#
# This is hosted by a Yarn.social pod {{ .InstanceName }} running yarnd {{ .SoftwareVersion.FullVersion }}
# Learn more about Yarn.social at https://yarn.social
#
# nick        = {{ .Profile.Username }}
# url         = {{ .Profile.URL }}
# avatar      = {{ .Profile.Avatar }}
# description = {{ .Profile.Tagline }}
#
# followers   = {{ if .Profile.ShowFollowers }}{{ len .Profile.Followers }}{{ end }}
# following   = {{ if .Profile.ShowFollowing }}{{ len .Profile.Following }}{{ end }}
#
{{- if .Profile.ShowFollowing }}
{{ range $nick, $url := .Profile.Following -}}
# follow = {{ $nick }} {{ $url }}
{{ end -}}
#
{{ end }}
`

// TwtxtHandler ...
func (s *Server) TwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		s.tasks.DispatchFunc(func() error {
			return s.cache.DetectPodFromRequest(r)
		})

		ctx := NewContext(s, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		fn, err := securejoin.SecureJoin(filepath.Join(s.config.Data, "feeds"), nick)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		twtxtUA, _ := ParseUserAgent(r.UserAgent())
		if ua, ok := twtxtUA.(*SingleUserAgent); ok {
			var (
				user       *User
				feed       *Feed
				err        error
				followedBy bool
			)

			if user, err = s.db.GetUser(nick); err == nil {
				followedBy = user.FollowedBy(ua.URL)
			} else if feed, err = s.db.GetFeed(nick); err == nil {
				followedBy = feed.FollowedBy(ua.URL)
			} else {
				log.WithError(err).Warnf("unable to load user or feed object for %s", nick)
			}

			if (user != nil) || (feed != nil) {
				if (s.config.Debug || ua.IsPublicURL()) && !followedBy {
					if _, err := AppendSpecial(
						s.config, s.db,
						twtxtBot,
						fmt.Sprintf(
							"FOLLOW: @<%s %s> from @<%s %s> using %s",
							nick, URLForUser(s.config.BaseURL, nick),
							ua.Nick, ua.URL, ua.Client,
						),
					); err != nil {
						log.WithError(err).Warnf("error appending special FOLLOW post")
					}

					if user != nil {
						user.AddFollower(ua.Nick, ua.URL)
						if err := s.db.SetUser(nick, user); err != nil {
							log.WithError(err).Warnf("error updating user object for %s", nick)
						}
					} else if feed != nil {
						feed.AddFollower(ua.Nick, ua.URL)
						if err := s.db.SetFeed(nick, feed); err != nil {
							log.WithError(err).Warnf("error updating feed object for %s", nick)
						}
					} else {
						panic("should not be reached")
						// Should not be reached
					}
				}
			}
		}

		// XXX: This is stupid doing this twice
		// TODO: Refactor all of this :/

		if user, err := s.db.GetUser(nick); err == nil {
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if feed, err := s.db.GetFeed(nick); err == nil {
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			log.WithError(err).Warnf("unable to load user or feed profile for %s", nick)
		}

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			log.WithError(err).Error("error calling Stat() on feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		pr, err := types.ReadPreambleFeed(f, stat.Size())
		if err != nil {
			log.WithError(err).Error("error reading feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		preampleTemplate := pr.Preamble()

		if preampleTemplate == "" {
			preampleCustomTemplateFn := filepath.Join(s.config.Data, feedsDir, fmt.Sprintf("%s.tpl", nick))
			if FileExists(preampleCustomTemplateFn) {
				if data, err := std_ioutil.ReadFile(preampleCustomTemplateFn); err == nil {
					preampleTemplate = string(data)
				} else {
					log.WithError(err).Warnf("error loading custom preamble template for %s", nick)
					preampleTemplate = defaultPreambleTemplate
				}
			}
		}

		if preampleTemplate == "" {
			preampleTemplate = defaultPreambleTemplate
		}

		preamble, err := RenderPlainText(preampleTemplate, ctx)
		if err != nil {
			log.WithError(err).Warn("error rendering twtxt preamble")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Link", fmt.Sprintf(`<%s/user/%s/webmention>; rel="webmention"`, s.config.BaseURL, nick))

		mrs := ioutil.NewMultiReadSeeker(strings.NewReader(preamble), pr)
		http.ServeContent(w, r, "", fileInfo.ModTime(), mrs)
	}
}
