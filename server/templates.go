package server

import (
	"fmt"
	"html/template"
)

func (s *Server) loadTemplates() map[string]*template.Template {
	layout := "templates/layout.html"
	m := map[string]*template.Template{
		"index":         template.Must(template.ParseFiles("templates/index.html", layout)),
		"editSong":      template.Must(template.ParseFiles("templates/edit_song.html", layout)),
		"newSong":       template.Must(template.New("base").ParseFiles("templates/new_song.html", layout)),
		"playVideo":     template.Must(template.New("base").Funcs(template.FuncMap{}).ParseFiles("templates/play_video.html", layout)),
		"editRfid":      template.Must(template.ParseFiles("templates/edit_rfid.html", layout)),
		"assignSong":    template.Must(template.ParseFiles("templates/assign_song.html", layout)),
		"raw":           template.Must(template.ParseFiles("templates/raw.html", layout)),
		"admin":         template.Must(template.ParseFiles("templates/admin.html", layout)),
		"adminEditSong": template.Must(template.ParseFiles("templates/editSong.html", layout)),
		"player":        template.Must(template.New("base").ParseFiles("templates/player.html", layout)),
		"print":         template.Must(template.New("base").ParseFiles("templates/print.html", layout)),
	}
	cfgMap := s.cfg.ToMap()
	configFuncs := template.FuncMap{
		"ConfigString": func(feature string) template.HTML {
			v := fmt.Sprint(cfgMap[feature])
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input id="%s" type="text" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
		"ConfigBool": func(feature string) template.HTML {
			checked := ""
			if v, ok := cfgMap[feature].(bool); ok && v {
				checked = "checked"
			}
			return template.HTML(fmt.Sprintf(`<input type="checkbox" name="%s" %s><i class="form-icon"></i> %s`, feature, checked, feature))
		},
		"ConfigInt": func(feature string) template.HTML {
			v := fmt.Sprint(cfgMap[feature])
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input class="form-input" id="%s" type="number" placeholder="00" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
	}
	m["config"] = template.Must(template.New("base").Funcs(configFuncs).ParseFiles("templates/config.html", layout))
	return m
}
