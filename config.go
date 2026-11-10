package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	CertificatesDir string `json:"certificadosDir"`
	ConvertedDir    string `json:"convertidosDir"`
	XMLBaseDir      string `json:"xmlBaseDir"`
	StatePath       string `json:"estadoPath"`
	ClientsCSV      string `json:"clientesCSV"`
	Address         string `json:"endereco"`
	UpdateSource    string `json:"atualizacaoExe"`
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

	if config.Address == "" {
		config.Address = "localhost:8080"
	}

	config.CertificatesDir = resolveDriveLetter(config.CertificatesDir)
	config.ConvertedDir = resolveDriveLetter(config.ConvertedDir)
	config.ClientsCSV = resolveDriveLetter(config.ClientsCSV)

	if err := os.MkdirAll(config.XMLBaseDir, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta %s: %w", config.XMLBaseDir, err)
	}

	controlDir := filepath.Dir(config.StatePath)
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return config, fmt.Errorf("não consegui criar a pasta de controle %s: %w", controlDir, err)
	}

	return config, nil
}

func retireOldLayout(config Config, now time.Time) (string, error) {
	if !hasOldLayout(config.XMLBaseDir) {
		return "", nil
	}

	stamp := now.Format("2006-01-02_150405")
	retired := config.XMLBaseDir + "-antigo-" + stamp
	if err := os.Rename(config.XMLBaseDir, retired); err != nil {
		return "", fmt.Errorf("a pasta %s está no formato antigo e não consegui renomeá-la "+
			"(feche as janelas abertas dentro dela e abra o programa de novo): %w", config.XMLBaseDir, err)
	}

	if !isInside(config.XMLBaseDir, config.StatePath) {
		if err := os.Rename(config.StatePath, config.StatePath+".antigo-"+stamp); err != nil && !errors.Is(err, os.ErrNotExist) {
			return retired, fmt.Errorf("renomeei as notas para %s, mas não consegui zerar o controle %s: %w",
				retired, config.StatePath, err)
		}
	}

	if err := os.MkdirAll(config.XMLBaseDir, 0o755); err != nil {
		return retired, fmt.Errorf("não consegui criar a pasta %s: %w", config.XMLBaseDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(config.StatePath), 0o755); err != nil {
		return retired, fmt.Errorf("não consegui criar a pasta de controle: %w", err)
	}

	return retired, nil
}

func hasOldLayout(baseDir string) bool {
	for _, company := range subdirs(baseDir) {
		for _, cnpj := range subdirs(company) {
			for _, month := range subdirs(cnpj) {
				if filepath.Base(month) == "_eventos" {
					continue
				}
				entries, _ := os.ReadDir(month)
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".xml") {
						return true
					}
				}
			}
		}
	}
	return false
}

func subdirs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths
}

func isInside(dir, path string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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

func resolveDriveLetter(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}

	if len(path) < 3 || path[1] != ':' {
		return path
	}

	for letter := 'C'; letter <= 'Z'; letter++ {
		candidate := string(letter) + path[1:]
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return path
}
