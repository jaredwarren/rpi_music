package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/jaredwarren/rpi_music/db"
)

func (s *Server) AdminEditSong(w http.ResponseWriter, r *http.Request) {
	songID := r.PathValue("song_id")
	song, err := s.db.GetSong(songID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.httpError(w, fmt.Errorf("song not found"), http.StatusNotFound)
			return
		}
		s.httpError(w, fmt.Errorf("AdminEditSong|GetSong|%w", err), http.StatusInternalServerError)
		return
	}

	fullData := map[string]any{
		"Song":      song,
		TemplateTag: template.HTML(""),
	}
	s.render(w, r, s.templates["adminEditSong"], fullData)
}

func (s *Server) AdminInsertSong(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) AdminUpdateSong(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) AdminDelete(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) AdminTODO(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) AdminHome(w http.ResponseWriter, r *http.Request) {
	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("AdminHome|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	rfids, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("AdminHome|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}
	enrichSongsWithRFID(songs, rfids)

	sort.Slice(songs, func(i, j int) bool {
		return songs[i].CreatedAt.Before(songs[j].CreatedAt)
	})

	fullData := map[string]any{
		"Songs":     songs,
		TemplateTag: template.HTML(""),
	}
	s.render(w, r, s.templates["admin"], fullData)
}
