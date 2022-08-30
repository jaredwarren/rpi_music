package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jaredwarren/rpi_music/log"
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
		default:
			s.logger.Warn("unknown ws command", log.Any("cmd", msg.Command))
		}

		// Write message back to browser
		if err = ws.WriteJSON(resp); err != nil {
			s.logger.Error(err.Error())
			continue
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
	// TODO: start downlaod ASYNC
	// push progress messages onto MessageChannel <- msg
	return resp
}
