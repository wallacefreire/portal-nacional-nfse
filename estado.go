package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ErrEstadoNaoSalvo sinaliza que o arquivo de controle não pôde ser gravado
var ErrEstadoNaoSalvo = errors.New("Não foi possível gravar o arquivo de controle")

// NSUState guarda o último NSU lido de cada CNPJ
type NSUState map[string]int64

func loadState(path string) (NSUState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NSUState{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o arquivo de estado: %w", err)
	}

	state := NSUState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("erro ao interpretar o JSON do estado: %w", err)
	}

	return state, nil
}

func saveState(path string, state NSUState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao gerar JSON do estado: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("erro ao salvar arquivo temporário do estado: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("erro ao renomear arquivo temporário do estado: %w", err)
	}

	return nil
}
