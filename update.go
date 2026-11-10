package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	updateCheckInterval = time.Hour
	updateSettleTime    = 2 * time.Minute
	minimumExeSize      = 1 << 20
	updatedEnv          = "PORTALNACIONAL_ATUALIZADO"
)

func watchForUpdates(config Config) {
	if config.UpdateSource == "" {
		return
	}

	exe, err := os.Executable()
	if err != nil || isTemporaryBuild(exe) {
		return
	}

	var checked time.Time
	for {
		time.Sleep(5 * time.Second)
		os.Remove(exe + ".antigo")

		source := resolveDriveLetter(config.UpdateSource)
		info, err := os.Stat(source)

		ready := err == nil &&
			!info.ModTime().Equal(checked) &&
			time.Since(info.ModTime()) > updateSettleTime &&
			!downloadRunning()

		if ready {
			updated, err := applyUpdate(source, exe)
			switch {
			case err != nil:
				log.Println("Atualização não aplicada:", err)
			case updated:
				log.Println("Nova versão instalada a partir de " + source + " - reiniciando")
				restart(exe)
			default:
				checked = info.ModTime()
			}
		}

		time.Sleep(updateCheckInterval - 5*time.Second)
	}
}

func applyUpdate(source, exe string) (bool, error) {
	incoming, err := os.ReadFile(source)
	if err != nil {
		return false, fmt.Errorf("lendo %s: %w", source, err)
	}

	current, err := os.ReadFile(exe)
	if err != nil {
		return false, fmt.Errorf("lendo o executável atual: %w", err)
	}

	if bytes.Equal(incoming, current) {
		return false, nil
	}

	if len(incoming) < minimumExeSize || !bytes.HasPrefix(incoming, []byte("MZ")) {
		return false, fmt.Errorf("%s não parece um executável completo (%d bytes)", source, len(incoming))
	}

	again, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(again, incoming) {
		return false, fmt.Errorf("%s mudou enquanto era lido - fica para a próxima verificação", source)
	}

	pending := exe + ".novo"
	if err := os.WriteFile(pending, incoming, 0o755); err != nil {
		return false, fmt.Errorf("gravando a versão nova: %w", err)
	}

	previous := exe + ".antigo"
	os.Remove(previous)

	if err := os.Rename(exe, previous); err != nil {
		os.Remove(pending)
		return false, fmt.Errorf("tirando o executável atual do lugar: %w", err)
	}

	if err := os.Rename(pending, exe); err != nil {
		os.Rename(previous, exe)
		return false, fmt.Errorf("colocando a versão nova no lugar: %w", err)
	}

	return true, nil
}

func restart(exe string) {
	cmd := exec.Command(exe, backgroundFlag)
	cmd.Env = append(os.Environ(), updatedEnv+"=1")

	if err := cmd.Start(); err != nil {
		log.Println("A versão nova foi instalada, mas não consegui reiniciar - ela entra na próxima vez que o programa abrir:", err)
		return
	}

	os.Exit(0)
}
