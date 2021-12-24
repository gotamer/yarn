package internal

import (
	"bufio"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// FollowHandler ...
func (s *Server) FollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := NormalizeURL(r.FormValue("url"))

		if r.Method == "GET" && nick == "" && url == "" {
			ctx.Title = s.tr(ctx, "PageFollowTitle")
			s.render("follow", w, ctx)
			return
		}

		if url == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoFeed")
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}
		trdata := map[string]interface{}{}
		trdata["Nick"] = nick
		trdata["URL"] = url
		if err := user.FollowAndValidate(s.config, nick, url); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorFollowAndValidate", trdata)
			s.render("error", w, ctx)
			return
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			trdata["Error"] = err.Error()
			ctx.Message = s.tr(ctx, "ErrorSetUser", trdata)
			s.render("error", w, ctx)
			return
		}

		s.cache.GetByUser(ctx.User, true)

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgFollowUserSuccess", trdata)
		s.render("error", w, ctx)
	}
}

// ImportHandler ...
func (s *Server) ImportHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			ctx.Title = "Import feeds from a list"
			s.render("import", w, ctx)
			return
		}

		feeds := r.FormValue("feeds")

		if feeds == "" {
			ctx.Error = true
			ctx.Message = "Nothing to import!"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		re := regexp.MustCompile(`(?P<nick>.*?)[: ](?P<url>.*)`)

		imported := 0

		scanner := bufio.NewScanner(strings.NewReader(feeds))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				nick := strings.TrimSpace(matches[1])
				url := NormalizeURL(strings.TrimSpace(matches[2]))
				if nick != "" && url != "" {
					user.Follow(nick, url)
					imported++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Error("error scanning feeds for import")
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
			return
		}

		s.cache.GetByUser(ctx.User, true)

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully imported %d feeds", imported)
		s.render("error", w, ctx)

	}
}

// UnfollowHandler ...
func (s *Server) UnfollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		nick := strings.TrimSpace(r.FormValue("nick"))

		if nick == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoNick")
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}
		trdata := map[string]interface{}{}
		url, ok := user.Following[nick]
		trdata["Nick"] = nick
		trdata["URL"] = url
		if !ok {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoFeedByNick", trdata)
			s.render("error", w, ctx)
			return
		}

		delete(user.Following, nick)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUnfollowingFeed", trdata)
			s.render("error", w, ctx)
			return
		}

		s.cache.GetByUser(ctx.User, true)

		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgUnfollowSuccess", trdata)
		s.render("error", w, ctx)
	}
}
