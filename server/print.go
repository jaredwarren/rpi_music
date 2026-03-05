package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func (s *Server) PrintHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	songID := vars["song_id"]

	song, err := s.db.GetSong(songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("PrintHandler|db.View|%w", err), http.StatusInternalServerError)
		return
	}

	thumbExists := true
	if _, err := os.Stat(song.Thumbnail); errors.Is(err, os.ErrNotExist) {
		thumbExists = false
	}

	if song.Thumbnail == "" || !thumbExists {
		v, err := s.downloader.GetVideo(song.URL)
		if err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|downloader.GetVideo|%w", err), http.StatusInternalServerError)
			return
		}
		thumb, err := s.downloader.DownloadThumb(v)
		if err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|downloader.DownloadThumb|%w", err), http.StatusInternalServerError)
			return
		}
		song.Thumbnail = thumb
		err = s.db.UpdateSong(song)
		if err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|db.Update|%w", err), http.StatusInternalServerError)
			return
		}
	}

	fullData := map[string]any{
		"Song":      song,
		TemplateTag: s.getCSRFField(),
	}
	s.render(w, r, s.templates["print"], fullData)
}
