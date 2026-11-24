package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Config struct {
	Token     string `json:"token"`
	ServerURL string `json:"server_url,omitempty"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, ".rune")

	// Create ~/.rune if it doesnt exists
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil

}

func load() (Config, string, error) {
	path, err := getConfigPath()
	if err != nil {
		return Config{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, path, nil // file doesn't exist yet — blank config
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func save(cfg Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0644)
}

func SaveToken(token string) error {
	cfg, _, err := load()
	if err != nil {
		return err
	}
	cfg.Token = token
	return save(cfg)
}

func SaveServerURL(serverURL string) error {
	cfg, _, err := load()
	if err != nil {
		return err
	}
	cfg.ServerURL = serverURL
	return save(cfg)
}

func GetServerURL(defaultPort string) string {
	cfg, _, _ := load()
	if cfg.ServerURL != "" {
		return cfg.ServerURL
	}
	return "http://localhost:" + defaultPort
}

func LoadToken() (string, error) {
	cfg, _, err := load()
	if err != nil {
		return "", err
	}
	if cfg.Token == "" {
		return "", os.ErrNotExist
	}
	return cfg.Token, nil
}
