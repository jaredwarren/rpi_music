package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	ConfigFile = "config"
	ConfigPath = "./config"
)

func InitConfig() {
	viper.SetConfigName(ConfigFile) // name of config file (without extension)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(ConfigPath)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			writeDefaultConfig()
		} else {
			panic(fmt.Errorf("Fatal error config file: %w \n", err))
		}
	}
}

func writeDefaultConfig() {
	f, err := os.Create(filepath.Join(ConfigPath, fmt.Sprintf("%s.yml", ConfigFile)))
	if err != nil {
		panic(err.Error())
	}
	f.Close()

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error new config file: %w \n", err))
	}

	SetDefaults()

	// Write config
	err = viper.WriteConfig()
	if err != nil {
		panic(fmt.Sprintf("writeDefaultConfig|WriteConfig|%s", err))
	}
}

func SetDefaults() {
	viper.Set("https", true)
	viper.Set("rfid-enabled", true)
	viper.Set("host", ":8000")
	viper.Set("startup.play", true)
	viper.Set("startup.file", "song_files/windows-xp-startup.mp3")

	viper.Set("beep", true)
	viper.Set("player.loop", false)
	viper.Set("player.volume", 100)
	viper.Set("restart", false)
	viper.Set("allow_override", true)
}
