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
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{
		"ConfigString": func(feature string) template.HTML {
			v := viper.GetString(feature)
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input id="%s" type="text" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
	}).ParseFiles(files...))
	render(w, r, tpl, fullData)
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: ConfigHandler ::")

	// get form.. update config, and write
	if false {
		viper.Set("beep", !viper.GetBool("beep"))
		viper.WriteConfig()
	}

}
