package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

const (
	TokenName   = "gorilla.csrf.Token"
	TemplateTag = "csrfField"
)

var csrf_store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func (s *Server) GetToken(w http.ResponseWriter, r *http.Request) template.HTML {
	token := genToken()

	session, err := store.Get(r, TokenName)
	if err != nil {
		return template.HTML("")
	}
	session.Values[TokenName] = token
	err = session.Save(r, w)
	if err != nil {
		return template.HTML("")
	}

	fragment := fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`,
		TokenName, token)

	return template.HTML(fragment)
}

func genToken() string {
	return uuid.New().String()
}

func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, CookieName)
		e, ok := session.Values[TokenName]
		if !ok {
			// for now just continue
			next.ServeHTTP(w, r)
			return
		}
		expectedToken := e.(string)
		if expectedToken == "" {
			// for now just continue
			next.ServeHTTP(w, r)
			return
		}

		// check query for all wss requests
		if r.URL.Scheme == "ws" || r.URL.Scheme == "wss" {
			queryToken := r.URL.Query().Get(TokenName)
			if queryToken == expectedToken {
				next.ServeHTTP(w, r)
				return
			}
		}

		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		// check form for token
		err := r.ParseForm()
		if err != nil {
			s.httpError(w, fmt.Errorf("requireCSRF|ParseForm|%w", err), http.StatusBadRequest)
			return
		}
		if r.PostForm.Has(TokenName) {
			formToken := r.PostForm.Get(TokenName)
			if formToken == expectedToken {
				next.ServeHTTP(w, r)
				return
			}
		}

		// check cookie for token
		tokenCookie, err := r.Cookie(TokenName)
		if err != nil {
			s.httpError(w, fmt.Errorf("requireCSRF|Cookie|%w", err), http.StatusBadRequest)
			return
		}
		if tokenCookie.String() == expectedToken {
			next.ServeHTTP(w, r)
			return
		}

		// Check query for token
		queryToken := r.URL.Query().Get(TokenName)
		if queryToken == expectedToken {
			next.ServeHTTP(w, r)
			return
		}

		s.httpError(w, fmt.Errorf("csrf token missing"), http.StatusForbidden)
		return
	})
}
