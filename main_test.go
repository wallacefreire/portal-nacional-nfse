package main

import "testing"

func TestOnlyDigits(t *testing.T) {
	resultado := onlyDigits("11.111.111/0001-91")
	esperado := "11111111000191"

	if resultado != esperado {
		t.Errorf("OnlyDigits deu %q, mas esperava %q", resultado, esperado)
	}
}

func TestExtractPassword(t *testing.T) {
	casos := []struct {
		nome     string
		esperado string
	}{
		{"EMPRESA EXEMPLO LTDA -- 11111111000191 -- senha abcd1234.pfx", "abcd1234"},
		{"COMERCIO MODELO Senha abcd1234.pfx", "abcd1234"},
		{"MERCADO FICTICIO ME -- 22222222000172 -- SENHA 998877.pfx", "998877"},
		{"JOAO EXEMPLO -- 33333333333 -- 55443322.pfx", "55443322"},
		{"TESTE E CIA LTDA -- 44444444000153 -- @9988.pfx", "@9988"},
	}

	for _, caso := range casos {
		resultado, err := extractPasswordFromFilename(caso.nome)
		if err != nil {
			t.Errorf("%q deu erro: %v", caso.nome, err)
			continue
		}
		if resultado != caso.esperado {
			t.Errorf("%q -> deu %q, esperava %q", caso.nome, resultado, caso.esperado)
		}
	}
}

func TestExtractIssueDate(t *testing.T) {
	xml := []byte(`<NFSe><dhProc>2026-07-29T16:01:25-03:00</dhProc></NFSe>`)

	resultado := extractIssueDate(xml)
	esperado := "2026-07"

	if resultado != esperado {
		t.Errorf("extractIssueDate deu %q, esperava %q", resultado, esperado)
	}
}

func TestPasswordCasosRuins(t *testing.T) {
	ruins := []string{
		"NOME SEM SEPARADOR 12345678901.pfx",
		"arquivo_qualquer.pfx",
		"",
	}

	for _, nome := range ruins {
		_, err := extractPasswordFromFilename(nome)
		if err == nil {
			t.Errorf("%q devia dar erro, mas passou", nome)
		}
	}
}
