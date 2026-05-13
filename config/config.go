package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFile = "config"
	ConfigPath = "./config"
	ConfigFull = ConfigPath + "/" + ConfigFile + ".yml"
)

// Duration wraps time.Duration so gopkg.in/yaml.v3 can parse "2s", "100ms", etc.
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == "" {
		return nil
	}
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("config: parse duration %q: %w", value.Value, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	if d.Duration == 0 {
		return "", nil
	}
	return d.String(), nil
}

// Config is the single source of truth for application configuration.
type Config struct {
	path string // unexported: set by Load, used by Save

	Host          string          `yaml:"host"`
	HTTPS         bool            `yaml:"https"`
	RFIDEnabled   bool            `yaml:"rfid-enabled"`
	Beep          bool            `yaml:"beep"`
	Restart       bool            `yaml:"restart"`
	AllowOverride bool            `yaml:"allow_override"`
	Downloader    string          `yaml:"downloader"`
	Player        PlayerConfig    `yaml:"player"`
	RFID          RFIDConfig      `yaml:"rfid"`
	Startup       StartupConfig   `yaml:"startup"`
	Log           LogConfig       `yaml:"log"`
	Localtunnel   LocaltunnelConfig `yaml:"localtunnel"`
}

type PlayerConfig struct {
	SongRoot  string `yaml:"song_root"`
	ThumbRoot string `yaml:"thumb_root"`
	Volume    int    `yaml:"volume"`
	Loop      bool   `yaml:"loop"`
}

type RFIDConfig struct {
	Cooldown       Duration `yaml:"cooldown"`
	PollInterval   Duration `yaml:"poll_interval"`
	ReadUIDTimeout Duration `yaml:"read_uid_timeout"`
	SPIPort        string   `yaml:"spi_port"`
	ResetPin       string   `yaml:"reset_pin"`
	IRQPin         string   `yaml:"irq_pin"`
}

// CooldownOrDefault returns the configured cooldown or 2s if unset.
func (r RFIDConfig) CooldownOrDefault() time.Duration {
	if r.Cooldown.Duration <= 0 {
		return 2 * time.Second
	}
	return r.Cooldown.Duration
}

// PollIntervalOrDefault returns the configured poll interval or 100ms if unset.
func (r RFIDConfig) PollIntervalOrDefault() time.Duration {
	if r.PollInterval.Duration <= 0 {
		return 100 * time.Millisecond
	}
	return r.PollInterval.Duration
}

// ReadUIDTimeoutOrDefault returns the configured read UID timeout or 5s if unset.
func (r RFIDConfig) ReadUIDTimeoutOrDefault() time.Duration {
	if r.ReadUIDTimeout.Duration <= 0 {
		return 5 * time.Second
	}
	return r.ReadUIDTimeout.Duration
}

type StartupConfig struct {
	Play bool   `yaml:"play"`
	File string `yaml:"file"`
}

type LogConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json or console
	File   string `yaml:"file"`
}

type LocaltunnelConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
}

// defaults returns the baseline Config used when no file exists or fields are missing.
func defaults() *Config {
	return &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Host:          ":8000",
		HTTPS:         true,
		RFIDEnabled:   true,
		Beep:          true,
		AllowOverride: true,
		Downloader:    "youtube-dl",
		Player: PlayerConfig{
			SongRoot:  "song_files",
			ThumbRoot: "thumb_files",
			Volume:    100,
		},
		Startup: StartupConfig{
			Play: true,
			File: "sounds/windows-xp-startup.mp3",
		},
	}
}

// Load reads the YAML config from path, applying defaults for any missing fields.
// If the file does not exist, the defaults are written to disk and returned.
func Load(path string) (*Config, error) {
	cfg := defaults()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, fmt.Errorf("config: mkdir: %w", err)
			}
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("config: write defaults: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	// Unmarshal on top of defaults so missing keys keep their default value.
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	cfg.path = path // yaml.Unmarshal overwrites unexported fields? No — it can't. Safe.
	return cfg, nil
}

// Save marshals the config and writes it to the path it was loaded from.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(c.path, data, 0o644)
}

// ToMap returns a flat string-keyed map compatible with the config template helpers.
func (c *Config) ToMap() map[string]any {
	return map[string]any{
		"https":                  c.HTTPS,
		"host":                   c.Host,
		"rfid-enabled":           c.RFIDEnabled,
		"beep":                   c.Beep,
		"restart":                c.Restart,
		"allow_override":         c.AllowOverride,
		"downloader":             c.Downloader,
		"player.song_root":       c.Player.SongRoot,
		"player.thumb_root":      c.Player.ThumbRoot,
		"player.volume":          c.Player.Volume,
		"player.loop":            c.Player.Loop,
		"startup.play":           c.Startup.Play,
		"startup.file":           c.Startup.File,
		"log.level":              c.Log.Level,
		"log.format":             c.Log.Format,
		"log.file":               c.Log.File,
		"localtunnel.enabled":    c.Localtunnel.Enabled,
		"localtunnel.host":       c.Localtunnel.Host,
		"rfid.cooldown":          c.RFID.Cooldown.String(),
		"rfid.poll_interval":     c.RFID.PollInterval.String(),
		"rfid.read_uid_timeout":  c.RFID.ReadUIDTimeout.String(),
	}
}
