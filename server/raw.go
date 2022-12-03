package server

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/spf13/viper"
)

func (s *Server) RawHandler(w http.ResponseWriter, r *http.Request) {
	// get everything from db,
	// get all files and images

	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	//Raw Files
	songfiles, err := ioutil.ReadDir(viper.GetString("player.song_root"))
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	sFiles := []string{}
	for _, file := range songfiles {
		if !file.IsDir() {
			sFiles = append(sFiles, file.Name())
		}
	}

	tfiles, err := ioutil.ReadDir(viper.GetString("player.thumb_root"))
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	thumbFiles := []string{}
	for _, file := range tfiles {
		if !file.IsDir() {
			thumbFiles = append(thumbFiles, file.Name())
		}
	}

	fullData := map[string]interface{}{
		"Songs":      songs,
		"SongFiles":  sFiles,
		"ThumbFiles": thumbFiles,
		TemplateTag:  s.GetToken(w, r),
	}

	// for now
	files := []string{
		"templates/raw.html",
		"templates/layout.html",
	}
	homepageTpl := template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}
