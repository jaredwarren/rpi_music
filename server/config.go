package server

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

func (s *Server) ConfigFormHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: ConfigFormHandler ::")

	push(w, "/static/style.css")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	song := &model.Song{
		ID: "new",
	}

	fullData := map[string]interface{}{
		"Song": song,
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
	render(w, r, tpl, fullData)
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: ConfigHandler ::")

	err := r.ParseForm()
	if err != nil {
		httpError(w, fmt.Errorf("ConfigHandler|ParseForm|%w", err))
		return
	}

	beep := r.PostForm.Get("beep")
	viper.Set("beep", beep == "on")

	loop := r.PostForm.Get("player.loop")
	viper.Set("player.loop", loop == "on")

	allow_override := r.PostForm.Get("allow_override")
	viper.Set("allow_override", allow_override == "on")

	volume := r.PostForm.Get("player.volume")
	viper.Set("player.volume", volume)

	startupSound := r.PostForm.Get("startup.sound")
	viper.Set("startup.sound", startupSound)

	// Write
	err = viper.WriteConfig()
	if err != nil {
		httpError(w, fmt.Errorf("ConfigHandler|WriteConfig|%w", err))
		return
	}

	http.Redirect(w, r, "/songs", 301)
}
