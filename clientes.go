package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type Company struct {
	Name  string
	Root  string
	CNPJs []string
}

func loadCompanies(path string) (map[string]*Company, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrindo o CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("lendo o CSV: %w", err)
	}

	companies := map[string]*Company{}

	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 3 {
			continue
		}

		cnpj := onlyDigits(row[1])
		name := limparNome(row[2])

		if len(cnpj) != 14 {
			continue
		}
		root := cnpj[:8]

		if companies[root] == nil {
			companies[root] = &Company{Name: name, Root: root}
		}
		if cnpj[8:12] == "0001" {
			companies[root].Name = name
		}
		companies[root].CNPJs = append(companies[root].CNPJs, cnpj)
	}

	return companies, nil
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

func limparNome(nome string) string {
	nome = strings.TrimSpace(nome)

	for _, sufixo := range []string{" - MATRIZ", " - FILIAL"} {
		if posicao := strings.Index(strings.ToUpper(nome), sufixo); posicao >= 0 {
			return strings.TrimSpace(nome[:posicao])
		}
	}

	return nome
}
