package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/spf13/viper"
)

const (
	ConfigFile = "config"
	ConfigPath = "./config"
)

// InitConfig load config file, write defaults if no file exists.
func InitConfig(logger log.Logger) {
	viper.SetConfigName(ConfigFile) // name of config file (without extension)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(ConfigPath)
	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			writeDefaultConfig(logger)
		} else {
			logger.Panic("error reading config", log.Error(err))
		}
	}
}

// writeDefaultConfig Set then write config file.
// should only run first time app is launched and no config file is found
func writeDefaultConfig(logger log.Logger) {
	fp := filepath.Join(ConfigPath, fmt.Sprintf("%s.yml", ConfigFile))
	logger.Info("writing default config", log.Any("file_path", fp))
	f, err := os.Create(fp)
	if err != nil {
		logger.Panic("error creating config file", log.Any("file_path", fp), log.Error(err))
	}
	defer f.Close()

	if err := viper.ReadInConfig(); err != nil {
		logger.Panic("error reading config file", log.Error(err))
	}

	SetDefaults()

	if err := viper.WriteConfig(); err != nil {
		logger.Panic("error writing config file", log.Error(err))
	}
}

// SetDefaults sets hard-coded default values
func SetDefaults() {
	viper.Set("https", true)
	viper.Set("rfid-enabled", true)
	viper.Set("host", ":8000")
	viper.Set("startup.play", true)
	viper.Set("startup.file", "sounds/windows-xp-startup.mp3")

	viper.Set("beep", true)
	viper.Set("player.loop", false)
	viper.Set("player.volume", 100)
	viper.Set("restart", false)
	viper.Set("allow_override", true)

	viper.Set("downloader", "youtube-dl")

	// RFID timing (durations: "2s", "100ms", etc.)
	viper.Set("rfid.cooldown", "2s")           // pause after handling a tag (or after GetSong error)
	viper.Set("rfid.poll_interval", "100ms")  // delay in ReadID loop between ReadUID attempts
	viper.Set("rfid.read_uid_timeout", "5s")  // timeout for each ReadUID call
}
