package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	CertificatesDir string `json:"certificadosDir"`
	ConvertedDir    string `json:"convertidosDir"`
	XMLBaseDir      string `json:"xmlBaseDir"`
	StatePath       string `json:"estadoPath"`
	ClientsCSV      string `json:"clientesCSV"`
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("lendo %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("erro ao interpretar o JSON da configuração %s: %w", path, err)
	}

	return config, nil
}

func applyDefaults(config Config) (Config, error) {
	if config.XMLBaseDir == "" || config.StatePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return config, fmt.Errorf("não consegui descobrir a pasta do usuário: %w", err)
		}

		base := filepath.Join(home, "Documents", "NFSE")

		if config.XMLBaseDir == "" {
			config.XMLBaseDir = base
		}
		if config.StatePath == "" {
			config.StatePath = filepath.Join(base, "_controle", "nsu.json")
		}
	}

	if err := os.MkdirAll(config.XMLBaseDir, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta %s: %w", config.XMLBaseDir, err)
	}

	controlDir := filepath.Dir(config.StatePath)
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta de controle %s: %w", controlDir, err)
	}

	return config, nil
}

func besideExe(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return name
	}
	return filepath.Join(filepath.Dir(exe), name)
}

func findConfig() string {
	beside := besideExe("config.json")
	if _, err := os.Stat(beside); err == nil {
		return beside
	}
	return "config.json"
}
