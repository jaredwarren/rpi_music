package server

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) PrintHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]

	song, err := s.db.GetSong(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err), http.StatusInternalServerError)
		return
	}

	fullData := map[string]interface{}{
		"Song": song,
	}
	files := []string{
		"templates/print.html",
		"templates/layout.html",
	}
	tpl := template.Must(template.New("base").ParseFiles(files...))
	s.render(w, r, tpl, fullData)
}
