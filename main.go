package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/jaredwarren/rpi_music/config"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/jaredwarren/rpi_music/rfid"
	"github.com/jaredwarren/rpi_music/server"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

const (
	DBPath = "my.db"
)

func main() {
	logger := log.NewStdLogger(log.Debug)

	// Init Config
	config.InitConfig(logger)
	// logger.SetLevel(log.Level(viper.GetInt64("log_level")))

	// override things that don't work on mac
	if runtime.GOOS == "darwin" {
		logger.Info("Disable Mac features.")
		viper.Set("https", false)
		viper.Set("rfid-enabled", false)
		viper.Set("beep", false)
	}

	// Init Player
	player.InitPlayer(logger)
	defer func() {
		player.Stop()
	}()

	// Init DB
	db, err := bolt.Open(DBPath, 0600, nil)
	if err != nil {
		logger.Panic("error opening db", log.Error(err))
	}
	defer db.Close()

	// Init RFID
	if viper.GetBool("rfid-enabled") {
		r := rfid.InitRFIDReader(db, logger)
		defer r.Close()
	}

	// Init Server
	htmlServer := server.StartHTTPServer(&server.Config{
		Host:         viper.GetString("host"),
		ReadTimeout:  35 * time.Second,
		WriteTimeout: 35 * time.Second,
		Db:           db,
	})
	defer htmlServer.StopHTTPServer()

	// Ready
	if viper.GetBool("startup.play") {
		go player.Play(&model.Song{
			FilePath: viper.GetString("startup.file"),
		})
	} else {
		go player.Beep()
	}

	// Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nmain : shutting down")
}
