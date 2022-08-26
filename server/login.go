package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"github.com/jaredwarren/rpi_music/log"
)

const (
	CookieName = "jwt"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func (s *Server) LoginForm(w http.ResponseWriter, r *http.Request) {
	fullData := map[string]interface{}{}

	// for now
	files := []string{
		"templates/login.html",
		"templates/layout.html",
	}
	homepageTpl = template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, CookieName)

	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("Login|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("Login", log.Any("form", r.PostForm))

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
	s.logger.Info("Login", log.Any("username", username), log.Any("password", password))

	if username != "asdf" || password != "asdf" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Set user as authenticated
	session.Values["authenticated"] = true
	session.Save(r, w)

	// 3. redirect to /
	http.Redirect(w, r, "/", 301)
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

		s.logger.Info("[AUTH]")
		session, _ := store.Get(r, CookieName)

		// Check if user is authenticated
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			s.logger.Info("[AUTH] access denied")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		s.logger.Info("[AUTH] access granted!!!")
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}
