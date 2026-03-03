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
