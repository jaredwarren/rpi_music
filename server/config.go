package server

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

func (s *Server) ConfigFormHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("ConfigFormHandler")

	song := model.NewSong()

	fullData := map[string]interface{}{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}

	files := []string{
		"templates/config.html",
		"templates/layout.html",
	}
	// TODO:  maybe these would be better as objects
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{
		"ConfigString": func(feature string) template.HTML {
			v := viper.GetString(feature)
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input id="%s" type="text" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
		"ConfigBool": func(feature string) template.HTML {
			v := viper.GetBool(feature)
			checked := ""
			if v {
				checked = `checked`
			}
			return template.HTML(fmt.Sprintf(`<input type="checkbox" name="%s" %s><i class="form-icon"></i> %s`, feature, checked, feature))
		},
		"ConfigInt": func(feature string) template.HTML {
			v := viper.GetInt(feature)
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input class="form-input" id="%s" type="number" placeholder="00" value="%d" name="%s">`, feature, feature, feature, v, feature))
		},
	}).ParseFiles(files...))
	s.render(w, r, tpl, fullData)
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
