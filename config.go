package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	CertificadosDir string `json:"certificadosDir"`
	ConvertidosDir  string `json:"convertidosDir"`
	XMLBaseDir      string `json:"xmlBaseDir"`
	EstadoPath      string `json:"estadoPath"`
	ClientesCSV     string `json:"clientesCSV"`
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

func aplicarPadroes(config Config) (Config, error) {
	if config.XMLBaseDir == "" || config.EstadoPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return config, fmt.Errorf("não consegui descobrir a pasta do usuário: %w", err)
		}

		base := filepath.Join(home, "Documents", "NFSE")

		if config.XMLBaseDir == "" {
			config.XMLBaseDir = base
		}
		if config.EstadoPath == "" {
			config.EstadoPath = filepath.Join(base, "_controle", "nsu.json")
		}
	}

	if err := os.MkdirAll(config.XMLBaseDir, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta %s: %w", config.XMLBaseDir, err)
	}

	pastaControle := filepath.Dir(config.EstadoPath)
	if err := os.MkdirAll(pastaControle, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta de controle %s: %w", pastaControle, err)
	}

	return config, nil
}
