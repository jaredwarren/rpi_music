package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
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
	// logger, err := log.NewFileLogger("logs.out")
	logger := log.NewStdLogger(log.Info)

	logger.Info("Starting RPi Music")

	// Init Config
	config.InitConfig(logger)
	logger.SetLevel(log.Level(viper.GetInt64("log.level")))
	logger.Info("Config initialized")

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

	// Shutdown on SIGINT (Ctrl+C) or SIGTERM (kill, systemd, Docker)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("main :shutting down", log.Any("signal", sig.String()))

	// Stop capturing signals so a second Ctrl+C uses default behavior (force exit)
	signal.Reset(os.Interrupt, syscall.SIGTERM)
}
