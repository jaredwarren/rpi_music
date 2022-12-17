package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
)

func (s *Server) EditRFIDSongFormHandler(w http.ResponseWriter, r *http.Request) {
	rfids, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("EditRFIDSongFormHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}
	rfidMap := map[string][]*model.Song{}
	for _, r := range rfids {
		rfidMap[r.RFID] = []*model.Song{}
		for _, sid := range r.Songs {
			song, err := s.db.GetSongV2(sid)
			if err != nil {
				s.httpError(w, fmt.Errorf("EditRFIDSongFormHandler|GetSongV2|%w", err), http.StatusBadRequest)
				return
			}
			rfidMap[r.RFID] = append(rfidMap[r.RFID], song)
		}
	}

	fullData := map[string]interface{}{
		"Rfids":     rfidMap,
		TemplateTag: s.GetToken(w, r),
	}

	files := []string{
		"templates/edit_rfid.html",
		"templates/layout.html",
	}
	editSongFormTpl := template.Must(template.ParseFiles(files...))
	s.render(w, r, editSongFormTpl, fullData)
}

func (s *Server) UnassignRFIDSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	songID := vars["song_id"]
	if songID == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

	rfid := vars["rfid"]
	if rfid == "" {
		s.httpError(w, fmt.Errorf("rfid required"), http.StatusBadRequest)
		return
	}

	s.logger.Warn("TODO:", log.Any("rfid", rfid), log.Any("song_id", songID))

	err := s.db.RemoveRFIDSong(rfid, songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("RemoveRFIDSong|%w", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"ok": true,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) AssignRFIDToSongFormHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

	song, err := s.db.GetSongV2(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("AssignRFIDToSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("AssignRFIDToSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	fullData := map[string]interface{}{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}

	files := []string{
		"templates/assign_song.html",
		"templates/layout.html",
	}
	editSongFormTpl := template.Must(template.ParseFiles(files...))
	s.render(w, r, editSongFormTpl, fullData)
}

func (s *Server) AssignRFIDToSongHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		s.httpError(w, fmt.Errorf("ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

	song, err := s.db.GetSongV2(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("AssignRFIDToSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("AssignRFIDToSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	// 3. Insert RFID if set
	rfid := r.PostForm.Get("rfid")
	fmt.Printf(" - - - -%+v\n", r.PostForm)
	rfid = strings.ReplaceAll(rfid, ":", "")
	if rfid == "" {
		s.httpError(w, fmt.Errorf("rfid required"), http.StatusInternalServerError)
		return
	}

	// Make sure rfid doesn't exist yet.
	rfidSong, err := s.db.GetRFIDSong(rfid)
	if err != nil {
		s.logger.Error("RFIDExists error", log.Error(err))
		s.httpError(w, fmt.Errorf("RFIDExists error %w", err), http.StatusInternalServerError)
		return
	} else if rfidSong != nil {
		s.httpError(w, fmt.Errorf("rfid aready assigned! (%+v)", rfidSong), http.StatusInternalServerError)
		return
	} // else continue

	err = s.db.AddRFIDSong(rfid, song.ID)
	if err != nil {
		s.logger.Error("AddRFIDSong error", log.Error(err))
		s.httpError(w, fmt.Errorf("AddRFIDSong error %w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}
