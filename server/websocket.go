package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
)

type Message struct {
	Command string            `json:"command"`
	Data    map[string]string `json:"data"`
	Error   string            `json:"error"`
}

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Poll file for changes with this period.
	filePeriod = 10 * time.Second
)

var (
	MessageChannel = make(chan *Message)
	upgrader       = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func (s *Server) reader(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		msg := &Message{}

		_, r, err := ws.NextReader()
		if err != nil {
			s.logger.Warn(err.Error())
			return
		}
		err = json.NewDecoder(r).Decode(msg)
		if err != nil {
			s.logger.Warn(err.Error())
			continue
		}

		// Print the message to the console
		s.logger.Info("ws message", log.Any("addr", ws.RemoteAddr()), log.Any("message", msg))

		var resp *Message
		switch msg.Command {
		case "player.play":
			resp = s.player_play(msg)
		case "player.stop":
			resp = s.player_stop(msg)
		case "download":
			resp = s.download(msg)
		case "toast":
			// used to test toast
			MessageChannel <- msg
		case "log":
			s.log(msg)
		default:
			s.logger.Warn("unknown ws command", log.Any("cmd", msg.Command))
		}

		// Write message back to browser
		if resp != nil {
			if err = ws.WriteJSON(resp); err != nil {
				s.logger.Error(err.Error())
				continue
			}
		}
	}
}

func (s *Server) writer(ws *websocket.Conn) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		ws.Close()
	}()
	for {
		select {
		case msg := <-MessageChannel:
			if msg != nil {
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteJSON(msg); err != nil {
					return
				}
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("HandleWS")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			s.logger.Error(err.Error())
		}
		return
	}

	go s.writer(ws)
	s.reader(ws)
}

func (s *Server) log(msg *Message) {
	switch msg.Data["level"] {
	case "error":
		s.logger.Error(msg.Data["message"])
	case "warn":
		s.logger.Warn(msg.Data["message"])
	case "info":
		s.logger.Info(msg.Data["message"])
	case "debug":
		s.logger.Debug(msg.Data["message"])
	default:
		s.logger.Warn("unknown log message", log.Any("msg", msg))
	}
}

func (s *Server) player_play(msg *Message) *Message {
	resp := &Message{
		Command: "player.play",
	}

	song, err := s.db.GetSong(msg.Data["song_id"])
	if err != nil {
		resp.Error = err.Error()
	}
	player.Beep()
	err = player.Play(song)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Data = map[string]string{
			"title": song.Title,
			"id":    song.ID,
			"thumb": song.Thumbnail,
		}
	}

	return resp
}

func (s *Server) player_stop(msg *Message) *Message {
	resp := &Message{
		Command: "player.stop",
	}
	player.Stop()
	return resp
}

func (s *Server) download(msg *Message) *Message {
	resp := &Message{
		Command: "download",
	}
	s.logger.Debug("Download.........", log.Any("msg", msg))

	url := msg.Data["url"]
	rfid := msg.Data["rfid"]
	resp.Data = map[string]string{
		"rfid": rfid,
		"url":  url,
	}

	go s.downloadVideo(rfid, url)

	return resp
}

func (s *Server) downloadVideo(rfid, url string) {
	rfid = strings.ReplaceAll(rfid, ":", "")

	MessageChannel <- &Message{
		Command: "toast",
		Data: map[string]string{
			"title": "Downloading Video",
			"text":  fmt.Sprintf("%s", url),
		},
	}

	song := &model.Song{
		ID:   rfid,
		URL:  url,
		RFID: rfid,
	}

	// try to download file again
	file, video, err := s.downloader.DownloadVideo(url, s.logger)
	if err != nil {
		MessageChannel <- &Message{
			Command: "error",
			Error:   fmt.Errorf("DownloadVideo|%w", err).Error(),
		}
	}

	MessageChannel <- &Message{
		Command: "toast",
		Data: map[string]string{
			"title": "Downloading Thumb",
			"text":  fmt.Sprintf("%s", video.Title),
		},
	}

	tmb, err := s.downloader.DownloadThumb(video)
	if err != nil {
		s.logger.Warn("UpdateSongHandler|downloadThumb", log.Error(err))
		// ignore err
	}

	song.Thumbnail = tmb
	song.FilePath = file
	song.Title = video.Title

	// Update otherwise
	err = s.db.UpdateSong(song)
	if err != nil {
		MessageChannel <- &Message{
			Command: "error",
			Error:   fmt.Errorf("db.Update|%w", err).Error(),
		}
		return
	}

	b, _ := json.Marshal(song)
	// ignore error for now
	MessageChannel <- &Message{
		Command: "download.done",
		Data:    map[string]string{"song": string(b)},
	}
}
