package main

import (
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/jaredwarren/rpi_music/config"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/localtunnel"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/jaredwarren/rpi_music/rfid"
	"github.com/jaredwarren/rpi_music/server"
	"github.com/spf13/viper"
)

const (
	DBPath = "my.db"
)

func main() {
	logger := log.NewStdLogger(log.Debug)

	// Init Config
	config.InitConfig(logger)
	logger.SetLevel(log.Level(viper.GetInt64("log.level")))

	// override things that don't work on mac
	if runtime.GOOS == "darwin" {
		logger.Info("Disable Mac features.")
		viper.Set("rfid-enabled", false)
	}

	// Localtunnel setup
	if viper.GetBool("localtunnel.enabled") {
		err := localtunnel.Init(logger)
		if err != nil {
			logger.Fatal(err.Error())
		}
		defer func() {
			_ = localtunnel.Close()
		}()
	}

	// Init Player
	logger.Debug("Init Player")
	player.InitPlayer(logger)
	defer func() {
		player.Stop()
	}()

	// Init DB
	logger.Debug("Init DB")
	sdb, err := db.NewSongDB(DBPath)
	if err != nil {
		logger.Panic("error opening db", log.Error(err))
	}
	defer sdb.Close()

	// Migrate DB
	logger.Debug("Migrage DB")
	db.Up(sdb)
	if err != nil {
		logger.Panic("error opening db", log.Error(err))
	}

	// Init RFID
	if viper.GetBool("rfid-enabled") {
		logger.Debug("Init RFID")
		r := rfid.InitRFIDReader(sdb, logger)
		defer r.Close()
	}

	// Init Server
	logger.Debug("Init Server")
	htmlServer := server.StartHTTPServer(&server.Config{
		Host:         viper.GetString("host"),
		ReadTimeout:  350 * time.Second,
		WriteTimeout: 350 * time.Second,
		Db:           sdb,
		Logger:       logger,
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

	logger.Info("Ready...")

	// Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	logger.Info("main :shutting down")
}
