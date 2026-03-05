package server

import (
	"fmt"
	"net/http"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

func (s *Server) ConfigFormHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("ConfigFormHandler")

	song := model.NewSong()

	fullData := map[string]any{
		"Song":      song,
		TemplateTag: s.getCSRFField(),
	}
	s.render(w, r, s.templates["config"], fullData)
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("ConfigHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("ConfigHandler", log.Any("form", r.PostForm))

	beep := r.PostForm.Get("beep")
	viper.Set("beep", beep == "on")

	loop := r.PostForm.Get("player.loop")
	viper.Set("player.loop", loop == "on")

	allow_override := r.PostForm.Get("allow_override")
	viper.Set("allow_override", allow_override == "on")

	volume := r.PostForm.Get("player.volume")
	viper.Set("player.volume", volume)

	startupSound := r.PostForm.Get("startup.play")
	viper.Set("startup.play", startupSound == "on")

	// Write
	err = viper.WriteConfig()
	if err != nil {
		s.httpError(w, fmt.Errorf("ConfigHandler|WriteConfig|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}
