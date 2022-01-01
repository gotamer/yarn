package internal

import (
	"fmt"
	std_ioutil "io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.mills.io/yarnsocial/yarn"
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
# nick        = {{ .Profile.Nick }}
# url         = {{ .Profile.URI }}
# avatar      = {{ .Profile.Avatar }}
# description = {{ .Profile.Description }}
#
# followers   = {{ if .Profile.ShowFollowers }}{{ .Profile.NFollowers }}{{ end }}
# following   = {{ if .Profile.ShowFollowing }}{{ .Profile.NFollowing }}{{ end }}
#
{{- if .Profile.ShowFollowing }}
{{ range $f := .Profile.Following -}}
# follow = {{ $f.Nick }} {{ $f.URI }}
{{ end -}}
#
{{ end }}
`

// TwtxtHandler ...
func (s *Server) TwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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

		if user, err := s.db.GetUser(nick); err == nil {
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
			followers := s.cache.GetFollowers(ctx.Profile)
			ctx.Profile.Followers = followers
			ctx.Profile.NFollowers = len(followers)
		} else if feed, err := s.db.GetFeed(nick); err == nil {
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
			followers := s.cache.GetFollowers(ctx.Profile)
			ctx.Profile.Followers = followers
			ctx.Profile.NFollowers = len(followers)
		} else {
			log.WithError(err).Warnf("unable to load user or feed profile for %s", nick)
		}

		s.tasks.DispatchFunc(func() error {
			return s.cache.DetectClientFromRequest(r, ctx.Profile)
		})

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
		w.Header().Set("Powered-By", fmt.Sprintf("yarnd/%s (Pod: %s Support: %s)", yarn.FullVersion(), s.config.Name, URLForPage(s.config.BaseURL, "support")))

		mrs := ioutil.NewMultiReadSeeker(strings.NewReader(preamble), pr)
		http.ServeContent(w, r, "", fileInfo.ModTime(), mrs)
	}
}
