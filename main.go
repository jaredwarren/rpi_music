package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/jaredwarren/rpi_music/rfid"
	"github.com/jaredwarren/rpi_music/server"
	bolt "go.etcd.io/bbolt"
)

const (
	DBPath      = "my.db"
	DoSSL       = true
	rfidEnabled = false
)

func main() {
	serverCfg := Config{
		Host:         ":8000",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// init DB
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open(DBPath, 0600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	serverCfg.db = db

	if rfidEnabled {
		rfid := StartRFIDReader(db)
		defer rfid.Close()
	}

	htmlServer := StartHTTPServer(serverCfg)
	defer htmlServer.StopHTTPServer()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nmain : shutting down")
}

func StartRFIDReader(db *bolt.DB) *rfid.RFIDReader {
	r, err := rfid.New(nil)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			id := r.ReadID()
			fmt.Println("Look for ID:", id)
			var song *model.Song
			db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(server.SongBucket))
				v := b.Get([]byte(id))
				if v == nil {
					return nil
				}
				err := json.Unmarshal(v, &song)
				if err != nil {
					return err
				}
				return nil
			})
			if song != nil {
				fmt.Printf("=== PLAY! === \n{%+v}\n", song)
				err := player.Play(song.FilePath)
				if err != nil {
					fmt.Println("::::[E]", err)
				}
			} else {
				fmt.Printf("Not found \n")
			}

			// cooldown
			time.Sleep(2 * time.Second)
		}
	}()

	return r
}

// Config provides basic configuration
type Config struct {
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	db           *bolt.DB
}

// HTMLServer represents the web service that serves up HTML
type HTMLServer struct {
	server *http.Server
	wg     sync.WaitGroup
}

// Start launches the HTML Server
func StartHTTPServer(cfg Config) *HTMLServer {
	// Setup Context
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init server
	s := server.New(cfg.db)

	// Setup Handlers
	r := mux.NewRouter()

	// list songs
	r.HandleFunc("/songs", s.ListSongHandler).Methods("GET")
	// new song form
	r.HandleFunc("/song/new", s.NewSongFormHandler).Methods("GET")
	// submit new song
	r.HandleFunc("/song", s.NewSongHandler).Methods("POST")
	r.HandleFunc("/song/new", s.NewSongHandler).Methods("POST")
	// Edit Song Form
	r.HandleFunc("/song/{song_id}", s.EditSongFormHandler).Methods("GET")
	// new link
	r.HandleFunc("/song/{song_id}", s.UpdateSongHandler).Methods("PUT", "POST")
	// delete song
	r.HandleFunc("/song/{song_id}", s.DeleteSongHandler).Methods("DELETE")
	r.HandleFunc("/song/{song_id}/delete", s.DeleteSongHandler).Methods("GET") // temp unitl I can get a better UI

	r.HandleFunc("/song/{song_id}/play", s.PlaySongHandler)
	r.HandleFunc("/song/{song_id}/stop", s.StopSongHandler)

	// maybe?
	// play locally or remotely
	// remote media controls (WS?) (play, pause, volume +/-)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/song_files/").Handler(http.StripPrefix("/song_files/", http.FileServer(http.Dir("./song_files"))))

	// Create the HTML Server
	htmlServer := HTMLServer{
		server: &http.Server{
			Addr:           cfg.Host,
			Handler:        r,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: 1 << 20,
		},
	}

	// Add to the WaitGroup for the listener goroutine
	htmlServer.wg.Add(1)

	// Start the listener
	go func() {
		fmt.Printf("\nHTMLServer : Service started : Host= http://%v\n", cfg.Host)
		if DoSSL {
			htmlServer.server.ListenAndServeTLS("localhost.crt", "localhost.key")
		} else {
			htmlServer.server.ListenAndServe()
		}
		htmlServer.wg.Done()
	}()

	return &htmlServer
}

// Stop turns off the HTML Server
func (htmlServer *HTMLServer) StopHTTPServer() error {
	// Create a context to attempt a graceful 5 second shutdown.
	const timeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("HTMLServer : Service stopping\n")

	// Attempt the graceful shutdown by closing the listener
	// and completing all inflight requests
	if err := htmlServer.server.Shutdown(ctx); err != nil {
		// Looks like we timed out on the graceful shutdown. Force close.
		if err := htmlServer.server.Close(); err != nil {
			fmt.Printf("\nHTMLServer : Service stopping : Error=%v\n", err)
			return err
		}
	}

	// Wait for the listener to report that it is closed.
	htmlServer.wg.Wait()
	fmt.Printf("HTMLServer : Stopped\n")
	return nil
}
