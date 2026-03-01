package server

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/log"
)

func (s *Server) AdminEditSong(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()
	logger.Info("AdminEditSong")

	vars := mux.Vars(r)
	songID := vars["song_id"]
	song, err := s.db.GetSong(songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err), http.StatusInternalServerError)
		return
	}

	fullData := map[string]any{
		"Song": song,
	}

	files := []string{
		"templates/editSong.html",
		"templates/layout.html",
	}
	homepageTpl := template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}

func (s *Server) AdminInsertSong(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()
	logger.Info("AdminInsertSong")

	vars := mux.Vars(r)
	songID := vars["song_id"]

	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("AdminInsertSong|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	if songID == "new" {
		// insert song here
	} else {
		// Update song
		song, err := s.db.GetSong(songID)
		if err != nil {
			s.httpError(w, fmt.Errorf("AdminInsertSong|db.View|%w", err), http.StatusInternalServerError)
			return
		}
		if song == nil {
			logger.Error("no song", log.Any("id", songID))
		}

		// update song
		thumb := r.Form.Get("thumb")
		fmt.Fprintf(w, "thumb:%s\n", thumb)
		title := r.Form.Get("title")
		fmt.Fprintf(w, "title:%s\n", title)
		url := r.Form.Get("url")
		fmt.Fprintf(w, "url:%s\n", url)
		filepath := r.Form.Get("filepath")
		fmt.Fprintf(w, "filepath:%s\n", filepath)

		plays := r.Form.Get("plays")
		fmt.Fprintf(w, "plays:%s\n", plays)
		// num, err := strconv.Atoi(str)
		// if err != nil {
		// 	s.httpError(w, fmt.Errorf("AdminInsertSong|plays invalid|%w", err), http.StatusInternalServerError)
		// 	return
		// } else {
		// 	fmt.Println("Converted number:", num)
		// }
		// song.Plays = plays

		created_at := r.Form.Get("created_at")
		fmt.Fprintf(w, "created_at:%s\n", created_at)
		updated_at := r.Form.Get("updated_at")
		fmt.Fprintf(w, "updated_at:%s\n", updated_at)

		song.Title = title
		song.Thumbnail = thumb
		song.URL = url
		song.FilePath = filepath
		// song.CreatedAt = created_at
		// song.UpdatedAt = updated_at

		// get and validate song info here

	}
	fmt.Fprintf(w, "TODO: finish insert/update:%s", songID)
}

func (s *Server) AdminUpdateSong(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()
	logger.Info("AdminUpdateSong")
}

func (s *Server) AdminDelete(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()
	logger.Info("AdminDelete")
}

func (s *Server) AdminTODO(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()
	logger.Info("AdminTODO")
}

func (s *Server) AdminHome(w http.ResponseWriter, r *http.Request) {
	logger := log.Get()

	logger.Info("AdminHome")

	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	rfids, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}
	for _, s := range songs {
		for _, r := range rfids {
			for _, rs := range r.Songs {
				if rs == s.ID {
					s.RFID = r.RFID
				}
			}
		}
	}

	sort.Slice(songs, func(i, j int) bool {
		return songs[i].CreatedAt.Before(songs[j].CreatedAt)
	})

	fullData := map[string]any{
		"Songs":     songs,
		TemplateTag: s.GetToken(w, r),
	}

	// for now
	files := []string{
		"templates/admin.html",
		"templates/layout.html",
	}
	homepageTpl := template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}
