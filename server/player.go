package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/jaredwarren/rpi_music/db"
)

// PlayerHandler renders the player status page.
func (s *Server) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.templates["player"], map[string]any{
		"Player":    s.player,
		"Song":      s.player.GetPlaying(),
		TemplateTag: template.HTML(""),
	})
}

// PlaySongHandler looks up the song by ID and starts playback.
func (s *Server) PlaySongHandler(w http.ResponseWriter, r *http.Request) {
	songID := r.PathValue("song_id")
	song, err := s.db.GetSong(songID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.httpError(w, fmt.Errorf("song not found"), http.StatusNotFound)
			return
		}
		s.httpError(w, fmt.Errorf("PlaySongHandler|GetSong|%w", err), http.StatusInternalServerError)
		return
	}

	if song.FilePath == "" {
		s.player.Error()
		s.httpError(w, fmt.Errorf("song has no file"), http.StatusBadRequest)
		return
	}

	s.player.Beep()
	if err := s.player.Play(song); err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|Play|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

// StopSongHandler stops the current playback.
func (s *Server) StopSongHandler(w http.ResponseWriter, r *http.Request) {
	s.player.Stop()
	http.Redirect(w, r, "/songs", http.StatusFound)
}
