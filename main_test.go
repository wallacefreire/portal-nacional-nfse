package main

import "testing"

func TestOnlyDigits(t *testing.T) {
	got := onlyDigits("11.111.111/0001-91")
	want := "11111111000191"

	if got != want {
		t.Errorf("OnlyDigits deu %q, mas esperava %q", got, want)
	}
}

func TestExtractPassword(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"EMPRESA EXEMPLO LTDA -- 11111111000191 -- senha abcd1234.pfx", "abcd1234"},
		{"COMERCIO MODELO Senha abcd1234.pfx", "abcd1234"},
		{"MERCADO FICTICIO ME -- 22222222000172 -- SENHA 998877.pfx", "998877"},
		{"JOAO EXEMPLO -- 33333333333 -- 55443322.pfx", "55443322"},
		{"TESTE E CIA LTDA -- 44444444000153 -- @9988.pfx", "@9988"},
	}

	for _, c := range cases {
		got, err := extractPasswordFromFilename(c.name)
		if err != nil {
			t.Errorf("%q deu erro: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q -> deu %q, esperava %q", c.name, got, c.want)
		}
	}
}

func TestExtractIssueDate(t *testing.T) {
	xml := []byte(`<NFSe><dhProc>2026-07-29T16:01:25-03:00</dhProc></NFSe>`)

	got := extractIssueDate(xml)
	want := "2026-07"

	if got != want {
		t.Errorf("extractIssueDate deu %q, esperava %q", got, want)
	}
}

func TestPasswordBadCases(t *testing.T) {
	bad := []string{
		"NOME SEM SEPARADOR 12345678901.pfx",
		"arquivo_qualquer.pfx",
		"",
	}

	for _, name := range bad {
		_, err := extractPasswordFromFilename(name)
		if err == nil {
			t.Errorf("%q devia dar erro, mas passou", name)
		}
	}
}
