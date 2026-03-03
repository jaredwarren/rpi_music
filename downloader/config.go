package downloader

import (
	"os/exec"

	"github.com/spf13/viper"
)

const (
	defaultSongDir  = "song_files"
	defaultThumbDir = "thumb_files"
)

// YoutubeDLConfig configures the yt-dlp CLI downloader. All fields are optional;
// empty values use defaults (DefaultYtDlpBinary, defaultSongDir, defaultThumbDir).
type YoutubeDLConfig struct {
	SongRoot  string // output dir for audio (default: "song_files" or viper player.song_root)
	ThumbRoot string // output dir for thumbnails (default: "thumb_files" or viper player.thumb_root)
	Binary    string // yt-dlp executable name (default: DefaultYtDlpBinary)
}

func (c *YoutubeDLConfig) songRoot() string {
	if c != nil && c.SongRoot != "" {
		return c.SongRoot
	}
	if s := viper.GetString("player.song_root"); s != "" {
		return s
	}
	return defaultSongDir
}

func (c *YoutubeDLConfig) thumbRoot() string {
	if c != nil && c.ThumbRoot != "" {
		return c.ThumbRoot
	}
	if s := viper.GetString("player.thumb_root"); s != "" {
		return s
	}
	return defaultThumbDir
}

func (c *YoutubeDLConfig) binary() string {
	if c != nil && c.Binary != "" {
		return c.Binary
	}
	return DefaultYtDlpBinary
}

// EnsureYtDlpAvailable returns ErrExecutableNotFound if the configured binary is not in PATH.
// Call at startup when using YoutubeDLDownloader to fail fast.
// cfg may be nil (uses DefaultYtDlpBinary).
func EnsureYtDlpAvailable(cfg *YoutubeDLConfig) error {
	if cfg == nil {
		cfg = &YoutubeDLConfig{}
	}
	if _, err := exec.LookPath(cfg.binary()); err != nil {
		return ErrExecutableNotFound
	}
	return nil
}
