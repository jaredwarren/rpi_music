package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/jaredwarren/rpi_music/model"
)

func (s *Server) ConfigFormHandler(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.templates["config"], map[string]any{
		"Song":      model.NewSong(),
		TemplateTag: template.HTML(""),
	})
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.ConfigHandlerE(w, r); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			s.httpError(w, httpErr.Err, httpErr.Code)
			return
		}
		s.httpError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) ConfigHandlerE(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return asHTTPError(http.StatusBadRequest, fmt.Errorf("ConfigHandler|ParseForm|%w", err))
	}
	s.logger.Info("ConfigHandler", "form", r.PostForm)

	s.cfg.Beep = r.PostForm.Get("beep") == "on"
	s.cfg.Player.Loop = r.PostForm.Get("player.loop") == "on"
	s.cfg.AllowOverride = r.PostForm.Get("allow_override") == "on"
	s.cfg.Startup.Play = r.PostForm.Get("startup.play") == "on"

	if v := r.PostForm.Get("player.volume"); v != "" {
		if vol, err := strconv.Atoi(v); err == nil {
			s.cfg.Player.Volume = vol
		}
	}

	if err := s.cfg.Save(); err != nil {
		return asHTTPError(http.StatusInternalServerError, fmt.Errorf("ConfigHandler|Save|%w", err))
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
	return nil
}
