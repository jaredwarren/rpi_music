package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
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
)

const DBPath = "my.db"

func main() {
	cfg, err := config.Load(config.ConfigFull)
	if err != nil {
		// Can't use logger yet — fall back to stderr.
		os.Stderr.WriteString("load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log.Init(log.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		File:   cfg.Log.File,
	})
	logger := log.Get()

	logger.Info("Starting RPi Music")
	logger.Info("Config initialized")

	if runtime.GOOS == "darwin" {
		logger.Info("Disabling RFID on macOS")
		cfg.RFIDEnabled = false
	}

	// Localtunnel
	if cfg.Localtunnel.Enabled {
		t, err := localtunnel.New(localtunnel.Config{
			Subdomain: cfg.Localtunnel.Host,
			AppHost:   cfg.Host,
		}, logger)
		if err != nil {
			logger.Error("localtunnel", "err", err)
			os.Exit(1)
		}
		defer t.Close()
	}

	// Player
	p, err := player.New(player.Config{
		SongRoot:      cfg.Player.SongRoot,
		ThumbRoot:     cfg.Player.ThumbRoot,
		Volume:        cfg.Player.Volume,
		Loop:          cfg.Player.Loop,
		AllowOverride: cfg.AllowOverride,
		Restart:       cfg.Restart,
		Beep:          cfg.Beep,
		FFPlayBin:     findFFPlay(),
	}, logger)
	if err != nil {
		if runtime.GOOS != "darwin" {
			logger.Error("player", "err", err)
			os.Exit(1)
		}
		logger.Warn("ffplay not found — playback disabled; install via: brew install ffmpeg", "err", err)
		trueBin, _ := exec.LookPath("true")
		p, _ = player.New(player.Config{FFPlayBin: trueBin, Beep: false}, logger)
	}
	defer p.Stop()

	// Database
	sdb, err := db.NewSongDB(DBPath)
	if err != nil {
		logger.Error("db", "err", err)
		os.Exit(1)
	}
	defer sdb.Close()

	// Application lifecycle context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	if cfg.RFIDEnabled {
		events := make(chan rfid.Event, 4)
		r, err := rfid.New(rfid.Config{
			SPIPort:        cfg.RFID.SPIPort,
			ResetPin:       cfg.RFID.ResetPin,
			IRQPin:         cfg.RFID.IRQPin,
			Cooldown:       cfg.RFID.CooldownOrDefault(),
			PollInterval:   cfg.RFID.PollIntervalOrDefault(),
			ReadUIDTimeout: cfg.RFID.ReadUIDTimeoutOrDefault(),
		}, events, logger)
		if err != nil {
			logger.Error("rfid", "err", err)
			os.Exit(1)
		}
		defer r.Close()
		r.Start(ctx)

		wg.Add(1)
		go func() {
			defer wg.Done()
			runRFIDLoop(ctx, events, sdb, p, logger)
		}()
	}

	// HTTP server
	htmlServer, err := server.StartHTTPServer(&server.Config{
		AppConfig:    cfg,
		Context:      ctx,
		ReadTimeout:  350 * time.Second,
		WriteTimeout: 350 * time.Second,
		Db:           sdb,
		Logger:       logger,
		Player:       p,
	})
	if err != nil {
		logger.Error("http server init", "err", err)
		os.Exit(1)
	}
	// Startup sound
	if cfg.Startup.Play && cfg.Startup.File != "" {
		go func() {
			_ = p.Play(&model.Song{FilePath: cfg.Startup.File})
		}()
	} else {
		go p.Beep()
	}

	logger.Info("Ready...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("shutting down", "signal", sig.String())
	signal.Reset(os.Interrupt, syscall.SIGTERM)
	cancel()
	if err := htmlServer.StopHTTPServer(); err != nil {
		logger.Error("http server shutdown", "err", err)
	}
	wg.Wait()
}

// findFFPlay returns the path to ffplay, checking Homebrew locations on macOS.
func findFFPlay() string {
	for _, c := range []string{"ffplay", "/opt/homebrew/bin/ffplay", "/usr/local/bin/ffplay"} {
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
	}
	return "ffplay"
}

// runRFIDLoop consumes tag events and triggers playback.
func runRFIDLoop(ctx context.Context, events <-chan rfid.Event, sdb db.DBer, p *player.Player, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			rs, err := sdb.GetRFIDSong(ev.UID)
			if err != nil {
				if !errors.Is(err, db.ErrNotFound) {
					logger.Error("rfid: GetRFIDSong", "err", err)
				}
				continue
			}
			if len(rs.Songs) == 0 {
				continue
			}
			song, err := sdb.GetSong(rs.Songs[0])
			if err != nil {
				logger.Error("rfid: GetSong", "err", err)
				continue
			}
			p.Beep()
			if err := p.Play(song); err != nil {
				logger.Error("rfid: Play", "err", err)
				continue
			}

			song.Plays++
			if err := sdb.UpdateSong(song); err != nil {
				logger.Error("rfid: UpdateSong", "err", err)
			}
		}
	}
}
