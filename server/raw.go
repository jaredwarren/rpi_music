package server

import (
	"fmt"
	"net/http"
	"os"

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

	rss, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	//Raw Files
	songfiles, err := os.ReadDir(viper.GetString("player.song_root"))
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

	tfiles, err := os.ReadDir(viper.GetString("player.thumb_root"))
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

	fullData := map[string]any{
		"Songs":      songs,
		"SongFiles":  sFiles,
		"RFIDSongs":  rss,
		"ThumbFiles": thumbFiles,
		TemplateTag:  s.getCSRFField(),
	}

	s.render(w, r, s.templates["raw"], fullData)
}
