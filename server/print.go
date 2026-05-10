package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
)

func (s *Server) PrintHandler(w http.ResponseWriter, r *http.Request) {
	songID := r.PathValue("song_id")
	song, err := s.db.GetSong(songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("PrintHandler|GetSong|%w", err), http.StatusInternalServerError)
		return
	}

	thumbMissing := song.Thumbnail == ""
	if !thumbMissing {
		if _, err := os.Stat(song.Thumbnail); errors.Is(err, os.ErrNotExist) {
			thumbMissing = true
		}
	}

	if thumbMissing {
		v, err := s.downloader.GetVideo(song.URL)
		if err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|GetVideo|%w", err), http.StatusInternalServerError)
			return
		}
		thumb, err := s.downloader.DownloadThumb(v)
		if err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|DownloadThumb|%w", err), http.StatusInternalServerError)
			return
		}
		song.Thumbnail = normalizeAssetPath(thumb, s.thumbAssetRoot())
		if err := s.db.UpdateSong(song); err != nil {
			s.httpError(w, fmt.Errorf("PrintHandler|UpdateSong|%w", err), http.StatusInternalServerError)
			return
		}
	}

	s.render(w, r, s.templates["print"], map[string]any{
		"Song":      song,
		TemplateTag: template.HTML(""),
	})
}
