package player

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/jaredwarren/rpi_music/model"
)

const ffplayBin = "ffplay"

// Logger is the minimal logger contract player depends on.
// Kept local to this package so callers can satisfy it with any implementation.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Config holds all tunable player settings.
type Config struct {
	SongRoot      string
	ThumbRoot     string
	Volume        int
	Loop          bool
	AllowOverride bool
	Restart       bool
	Beep          bool
	FFPlayBin     string // defaults to "ffplay"
}

func (c Config) binary() string {
	if c.FFPlayBin != "" {
		return c.FFPlayBin
	}
	return ffplayBin
}

// Player manages a single ffplay subprocess.
type Player struct {
	cfg    Config
	logger Logger
	mu     sync.Mutex
	state  *playState
}

type playState struct {
	song *model.Song
	cmd  *exec.Cmd
}

// New creates a Player, validates that ffplay exists, and ensures song/thumb directories exist.
func New(cfg Config, logger Logger) (*Player, error) {
	if _, err := exec.LookPath(cfg.binary()); err != nil {
		return nil, fmt.Errorf("player: %s not found in PATH: %w", cfg.binary(), err)
	}
	for _, dir := range []string{cfg.SongRoot, cfg.ThumbRoot} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("player: create directory %s: %w", dir, err)
		}
	}
	return &Player{cfg: cfg, logger: logger}, nil
}

// Play starts playing song. If a song is already playing, behaviour depends on cfg.AllowOverride.
func (p *Player) Play(song *model.Song) error {
	if song == nil || song.FilePath == "" {
		return fmt.Errorf("song file path is empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != nil && p.state.song != nil && p.state.song.FilePath == song.FilePath && !p.cfg.Restart {
		p.logger.Info("selected song already playing", "song", song)
		return nil
	}
	if p.state != nil {
		if !p.cfg.AllowOverride {
			p.logger.Info("another song already playing", "song", song)
			p.playSound("sounds/error.wav")
			return nil
		}
		p.killLocked()
	}

	args := p.buildArgs(song.FilePath)
	p.logger.Info("Play song", "song", song, "args", args)

	cmd := exec.Command(p.cfg.binary(), args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffplay: %w", err)
	}

	st := &playState{song: song, cmd: cmd}
	p.state = st

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.state == st {
			p.state = nil
		}
	}()

	return nil
}

// Stop stops the current playback.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.killLocked()
}

func (p *Player) killLocked() {
	if p.state != nil && p.state.cmd != nil && p.state.cmd.Process != nil {
		_ = p.state.cmd.Process.Kill()
	}
	p.state = nil
}

// GetPlaying returns the currently playing song, or nil.
func (p *Player) GetPlaying() *model.Song {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == nil {
		return nil
	}
	return p.state.song
}

// Playing reports whether audio is currently playing.
func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state != nil
}

// Beep plays the success sound if beep is enabled.
func (p *Player) Beep() { p.playSound("sounds/success.wav") }

// Error plays the error sound if beep is enabled.
func (p *Player) Error() { p.playSound("sounds/error.wav") }

func (p *Player) playSound(path string) {
	if !p.cfg.Beep {
		return
	}
	args := p.buildArgs(path)
	cmd := exec.Command(p.cfg.binary(), args...)
	_ = cmd.Run()
}

func (p *Player) buildArgs(filePath string) []string {
	vol := p.cfg.Volume
	if vol <= 0 {
		vol = 100
	}
	args := []string{"-nodisp", "-autoexit"}
	args = append(args, "-volume", fmt.Sprintf("%d", vol))
	args = append(args, filePath)
	return args
}
