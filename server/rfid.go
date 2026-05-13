package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/model"
)

func (s *Server) EditRFIDSongFormHandler(w http.ResponseWriter, r *http.Request) {
	rfids, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("EditRFIDSongFormHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}
	rfidMap := map[string][]*model.Song{}
	for _, entry := range rfids {
		rfidMap[entry.RFID] = []*model.Song{}
		for _, sid := range entry.Songs {
			song, err := s.db.GetSong(sid)
			if err != nil {
				s.httpError(w, fmt.Errorf("EditRFIDSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
				return
			}
			rfidMap[entry.RFID] = append(rfidMap[entry.RFID], song)
		}
	}
	s.render(w, r, s.templates["editRfid"], map[string]any{
		"Rfids":     rfidMap,
		TemplateTag: template.HTML(""),
	})
}

func (s *Server) UnassignRFIDSongHandler(w http.ResponseWriter, r *http.Request) {
	songID := r.PathValue("song_id")
	if songID == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}
	rfid := r.PathValue("rfid")
	if rfid == "" {
		s.httpError(w, fmt.Errorf("rfid required"), http.StatusBadRequest)
		return
	}

	if err := s.db.RemoveRFIDSong(rfid, songID); err != nil {
		s.httpError(w, fmt.Errorf("UnassignRFIDSongHandler|RemoveRFIDSong|%w", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) AssignRFIDToSongFormHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("song_id")
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}
	song, err := s.db.GetSong(key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.httpError(w, fmt.Errorf("song not found"), http.StatusNotFound)
			return
		}
		s.httpError(w, fmt.Errorf("AssignRFIDToSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	s.render(w, r, s.templates["assignSong"], map[string]any{
		"Song":      song,
		TemplateTag: template.HTML(""),
	})
}

func (s *Server) AssignRFIDToSongHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.httpError(w, fmt.Errorf("ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	key := r.PathValue("song_id")
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}
	song, err := s.db.GetSong(key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.httpError(w, fmt.Errorf("song not found"), http.StatusNotFound)
			return
		}
		s.httpError(w, fmt.Errorf("AssignRFIDToSongHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	rfid := strings.ReplaceAll(r.PostForm.Get("rfid"), ":", "")
	if rfid == "" {
		s.httpError(w, fmt.Errorf("rfid required"), http.StatusBadRequest)
		return
	}

	rfidSong, err := s.db.GetRFIDSong(rfid)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		s.logger.Error("AssignRFIDToSongHandler|GetRFIDSong", "err", err)
		s.httpError(w, fmt.Errorf("RFIDExists error: %w", err), http.StatusInternalServerError)
		return
	}
	if rfidSong != nil {
		s.httpError(w, fmt.Errorf("rfid already assigned (%+v)", rfidSong), http.StatusConflict)
		return
	}

	if err := s.db.AddRFIDSong(rfid, song.ID); err != nil {
		s.logger.Error("AssignRFIDToSongHandler|AddRFIDSong", "err", err)
		s.httpError(w, fmt.Errorf("AddRFIDSong: %w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}
