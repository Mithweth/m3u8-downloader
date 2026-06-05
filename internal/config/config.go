package config

import (
	"errors"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
)

type VideosConfig struct {
	MaxParallel     int    `toml:"max_parallel"`
	PreferredFormat string `toml:"preferred_format"`
}

type PathsConfig struct {
	FFmpegBinary string `toml:"ffmpeg_binary"`
}

type TomlConfig struct {
	Videos  VideosConfig      `toml:"videos"`
	Paths   PathsConfig       `toml:"paths"`
	Headers map[string]string `toml:"headers"`
}

type Config struct {
	URL                string
	OutputFile         string
	TemporaryDirectory string
	FileConfig         TomlConfig
}

func getConfigFileName() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "m3u8-downloader")

	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(appDir, "config.toml"), nil
}

func LoadConfig() (TomlConfig, error) {
	cfg := TomlConfig{
		Videos: VideosConfig{
			MaxParallel: 3,
		},
		Headers: map[string]string{},
	}

	configFile, err := getConfigFileName()
	if err != nil {
		return TomlConfig{}, err
	}

	if _, err := os.Stat(configFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return TomlConfig{}, err
	}

	if _, err := toml.DecodeFile(configFile, &cfg); err != nil {
		return TomlConfig{}, err
	}

	return cfg, nil
}

func SaveConfig(cfg TomlConfig) error {
	configFile, err := getConfigFileName()
	if err != nil {
		return err
	}

	fd, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = fd.Close()
	}()

	return toml.NewEncoder(fd).Encode(cfg)
}
