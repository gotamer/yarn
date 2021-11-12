package internal

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"git.mills.io/yarnsocial/yarn/internal/session"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	// MaxFailedLogins is the default maximum tolerable number of failed login attempts
	// TODO: Make this configurable via Pod Settings
	MaxFailedLogins = 3 // By default 3 failed login attempts per 5 minutes
)

// LoginHandler ...
func (s *Server) LoginHandler() httprouter.Handle {
	// #239: Throttle failed login attempts and lock user  account.
	failures := NewTTLCache(5 * time.Minute)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			ctx.Title = s.tr(ctx, "LoginTitle")
			s.render("login", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		rememberme := r.FormValue("rememberme") == "on"

		// Error: no username or password provided
		if username == "" || password == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Lookup user
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidUsername")
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		if failures.Get(user.Username) > MaxFailedLogins {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorMaxFailedLogins")
			s.render("error", w, ctx)
			return
		}

		// Validate cleartext password against KDF hash
		err = s.pm.CheckPassword(user.Password, password)
		if err != nil {
			// #239: Throttle failed login attempts and lock user  account.
			failed := failures.Inc(user.Username)
			time.Sleep(time.Duration(IntPow(2, failed)) * time.Second)

			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidPassword")
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		failures.Reset(user.Username)

		// Lookup session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Authorize session
		_ = sess.(*session.Session).Set("username", username)

		// Persist session?
		if rememberme {
			_ = sess.(*session.Session).Set("persist", "1")
		}

		http.Redirect(w, r, RedirectRefererURL(r, s.config, "/"), http.StatusFound)
	}
}

// LoginEmailHandler ...
func (s *Server) LoginEmailHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		if r.Method == "GET" {
			ctx.Title = s.tr(ctx, "LoginEmailTitle")
			s.render("login_email", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		email := strings.TrimSpace(r.FormValue("email"))
		recovery := fmt.Sprintf("email:%s", FastHashString(email))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		// Check if user exist
		if !s.db.HasUser(username) {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserNotFound")
			s.render("error", w, ctx)
			return
		}

		// Get user object from DB
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorGetUser")
			s.render("error", w, ctx)
			return
		}

		if recovery != user.Recovery {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorUserRecovery")
			s.render("error", w, ctx)
			return
		}

		// Create magic link with a short expiry time of ~10m (hard-coded)
		// TOOD: Make the expiry time configurable?
		now := time.Now()
		secs := now.Unix()
		expiresAfterSeconds := int64(600) // Link expires after 10 minutes

		expiryTime := secs + expiresAfterSeconds

		// Create magic link
		token := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			jwt.MapClaims{"username": username, "expiresAt": expiryTime},
		)
		tokenString, err := token.SignedString([]byte(s.config.MagicLinkSecret))
		if err != nil {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		if err := SendMagicLinkAuthEmail(s.config, user, email, tokenString); err != nil {
			log.WithError(err).Errorf("unable to send magic-link-auth email to %s", user.Username)
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		// Show success msg
		ctx.Error = false
		ctx.Message = s.tr(ctx, "MsgMagicLinkAuthEmailSent")
		s.render("error", w, ctx)
	}
}

// MagicLinkAuthHandler ...
func (s *Server) MagicLinkAuthHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)

		// Get token from query string
		tokens, ok := r.URL.Query()["token"]

		// Check if valid token
		if !ok || len(tokens[0]) < 1 {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidToken")
			s.render("error", w, ctx)
			return
		}

		magicLinkAuthToken := tokens[0]

		// Check if token is valid
		token, err := jwt.Parse(magicLinkAuthToken, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(s.config.MagicLinkSecret), nil
		})
		if err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidToken")
			s.render("error", w, ctx)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			var username = fmt.Sprintf("%v", claims["username"])
			var expiresAt int = int(claims["expiresAt"].(float64))

			now := time.Now()
			secs := now.Unix()

			// Check token expiry
			if secs > int64(expiresAt) {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "errortokenexpires")
				s.render("error", w, ctx)
				return
			}

			user, err := s.db.GetUser(username)
			if err != nil {
				ctx.Error = true
				ctx.Message = s.tr(ctx, "ErrorGetUser")
				s.render("error", w, ctx)
				return
			}

			// Lookup session
			sess := r.Context().Value(session.SessionKey)
			if sess == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Authorize session
			_ = sess.(*session.Session).Set("username", user.Username)

			// Persist session?
			_ = sess.(*session.Session).Set("persist", "1")

			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorInvalidToken")
			s.render("error", w, ctx)
			return
		}
	}
}
