package player

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

const (
	ffplayBin = "ffplay"
)

var (
	instance *Player
	once     sync.Once
	logger   log.Logger
)

// Player holds the current playback state and process.
type Player struct {
	mu          sync.Mutex
	Playing     bool
	currentSong *model.Song
	cmd         *exec.Cmd
}

// InitPlayer initializes the player and ensures song/thumb directories exist.
func InitPlayer(l log.Logger) {
	logger = l
	for _, key := range []string{"player.song_root", "player.thumb_root"} {
		dir := viper.GetString(key)
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			logger.Panic("create dir", log.Any("dir", dir), log.Error(err))
		}
	}
}

// GetPlayer returns the singleton player instance.
func GetPlayer() *Player {
	once.Do(func() { instance = &Player{} })
	return instance
}

// Play starts playing the given song. If something is already playing, behavior
// depends on config (restart, allow_override). Returns an error if the song has
// no file path or ffplay fails to start.
func Play(song *model.Song) error {
	if song == nil || song.FilePath == "" {
		return fmt.Errorf("song file path is empty")
	}

	cp := GetPlayer()
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.Playing && cp.currentSong != nil && cp.currentSong.FilePath == song.FilePath && !viper.GetBool("restart") {
		logger.Info("selected song already playing", log.Any("song", song))
		return nil
	}
	if cp.Playing {
		if !viper.GetBool("allow_override") {
			logger.Info("another song already playing", log.Any("song", song))
			Error()
			return nil
		}
		cp.killAndClearLocked()
	}

	args := buildFFPlayArgs(song.FilePath)
	logger.Info("Play song", log.Any("song", song), log.Any("args", args))

	cmd := exec.Command(ffplayBin, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffplay: %w", err)
	}

	cp.cmd = cmd
	cp.Playing = true
	cp.currentSong = song

	go func() {
		_ = cmd.Wait()
		cp.mu.Lock()
		defer cp.mu.Unlock()
		// Only clear if this process is still the active one (not replaced by a later Play).
		if cp.cmd == cmd {
			cp.cmd = nil
			cp.Playing = false
			cp.currentSong = nil
		}
	}()

	return nil
}

// Stop stops the current playback and clears state.
func Stop() {
	p := GetPlayer()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.killAndClearLocked()
}

// killAndClearLocked kills the current process and clears state. Caller must hold p.mu.
func (p *Player) killAndClearLocked() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	p.cmd = nil
	p.Playing = false
	p.currentSong = nil
}

// GetPlaying returns the currently playing song, or nil.
func GetPlaying() *model.Song {
	cp := GetPlayer()
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.currentSong
}

// Beep plays the success sound if the beep config is enabled.
func Beep() {
	playSoundFile("sounds/success.wav")
}

// Error plays the error sound if the beep config is enabled.
func Error() {
	playSoundFile("sounds/error.wav")
}

func playSoundFile(path string) {
	if !viper.GetBool("beep") {
		return
	}
	args := buildFFPlayArgs(path)
	cmd := exec.Command(ffplayBin, args...)
	_ = cmd.Run()
}

func buildFFPlayArgs(filePath string) []string {
	args := []string{"-nodisp", "-autoexit"}
	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("player.volume")))
	args = append(args, filePath)
	return args
}
