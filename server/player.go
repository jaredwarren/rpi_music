package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/player"
)

// PlayerHandler renders the player status page.
func (s *Server) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	cp := player.GetPlayer()
	song := player.GetPlaying()

	fullData := map[string]any{
		"Player":    cp,
		"Song":      song,
		TemplateTag: s.getCSRFField(),
	}
	s.render(w, r, s.templates["player"], fullData)
}

// PlaySongHandler looks up the song by ID and starts playback.
func (s *Server) PlaySongHandler(w http.ResponseWriter, r *http.Request) {
	songID := mux.Vars(r)["song_id"]
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
		player.Error()
		s.httpError(w, fmt.Errorf("song has no file"), http.StatusBadRequest)
		return
	}

	player.Beep()
	if err := player.Play(song); err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|player.Play|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

// StopSongHandler stops the current playback.
func (s *Server) StopSongHandler(w http.ResponseWriter, r *http.Request) {
	player.Stop()
	http.Redirect(w, r, "/songs", http.StatusFound)
}
