package server

import (
	"fmt"
	"html/template"
	"net/http"
)

func (s *Server) RawHandler(w http.ResponseWriter, r *http.Request) {
	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("RawHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	rss, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("RawHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}

	s.render(w, r, s.templates["raw"], map[string]any{
		"Songs":      songs,
		"SongFiles":  readDir(s.cfg.Player.SongRoot),
		"RFIDSongs":  rss,
		"ThumbFiles": readDir(s.cfg.Player.ThumbRoot),
		TemplateTag:  template.HTML(""),
	})
}
