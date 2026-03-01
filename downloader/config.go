package downloader

import "github.com/spf13/viper"

const (
	defaultSongDir  = "song_files"
	defaultThumbDir = "thumb_files"
)

func getSongRoot() string {
	if s := viper.GetString("player.song_root"); s != "" {
		return s
	}
	return defaultSongDir
}

func getThumbRoot() string {
	if s := viper.GetString("player.thumb_root"); s != "" {
		return s
	}
	return defaultThumbDir
}
