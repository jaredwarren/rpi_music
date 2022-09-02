package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/jaredwarren/rpi_music/log"
)

const (
	CookieName = "jwt"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func (s *Server) LoginForm(w http.ResponseWriter, r *http.Request) {
	fullData := map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	}
	s.logger.Debug("login form", log.Any("data", fullData))

	files := []string{
		"templates/login.html",
		"templates/layout.html",
	}
	homepageTpl := template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second)

	session, _ := store.Get(r, CookieName)

	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("Login|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	username := r.PostForm.Get("username")
	if username == "" {
		s.httpError(w, fmt.Errorf("need username"), http.StatusBadRequest)
		return
	}

	password := r.PostForm.Get("password")
	if password == "" {
		s.httpError(w, fmt.Errorf("need password"), http.StatusBadRequest)
		return
	}

	expectedUsername := os.Getenv("USERNAME")
	expectedPassword := os.Getenv("PASSWORD")
	if username != expectedUsername || password != expectedPassword {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Set user as authenticated
	session.Values["authenticated"] = true
	session.Save(r, w)

	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, CookieName)

	// Revoke users authentication
	session.Values["authenticated"] = false
	session.Save(r, w)
	http.Redirect(w, r, "/login", 301)
}

func (s *Server) requireLoginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, CookieName)
		// TODO: fix this
		// bypass auth on wss:... because it doesn't work on Android
		if !strings.HasPrefix(r.RequestURI, "/echo") {
			if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
				s.logger.Warn("[AUTH] access denied", log.Any("req", r))
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
