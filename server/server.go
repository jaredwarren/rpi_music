package server

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"sync"

	"github.com/jaredwarren/rpi_music/config"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/player"
)

// Server is the application handler with all dependencies injected.
type Server struct {
	ctx          context.Context
	cfg          *config.Config
	db           Store
	logger       *slog.Logger
	downloader   downloader.Downloader
	player       *player.Player
	templates    map[string]*template.Template
	notifySubsMu sync.Mutex
	notifySubs   map[chan notifyEvent]struct{}
}

// New constructs a Server with all dependencies.
func New(ctx context.Context, cfg *config.Config, database Store, p *player.Player, l *slog.Logger) (*Server, error) {
	var dl downloader.Downloader
	if cfg.Downloader == "ytdl" {
		dl = &downloader.YoutubeDownloader{
			SongRoot:  cfg.Player.SongRoot,
			ThumbRoot: cfg.Player.ThumbRoot,
		}
		l.Info("using 'ytdl' downloader")
	} else {
		dlCfg := &downloader.YoutubeDLConfig{
			SongRoot:  cfg.Player.SongRoot,
			ThumbRoot: cfg.Player.ThumbRoot,
		}
		ytdl := downloader.NewYoutubeDLDownloader(dlCfg)
		if err := ytdl.EnsureAvailable(); err != nil {
			return nil, fmt.Errorf("server: youtube-dl backend unavailable: %w", err)
		}
		l.Info("using 'youtube-dl' downloader", "backend", ytdl.BackendDescription())
		dl = ytdl
	}

	srv := &Server{
		ctx:        ctx,
		cfg:        cfg,
		db:         database,
		logger:     l,
		downloader: dl,
		player:     p,
		notifySubs: make(map[chan notifyEvent]struct{}),
	}
	srv.templates = srv.loadTemplates()
	return srv, nil
}
