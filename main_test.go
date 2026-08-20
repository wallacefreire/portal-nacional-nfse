package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestLoadCompanies(t *testing.T) {
	companies, err := loadCompanies("clientes.exemplo.csv")
	if err != nil {
		t.Fatal(err)
	}

	if len(companies) != 8 {
		t.Errorf("deu %d raízes, esperava 8", len(companies))
	}

	group := companies["33333333"]
	if group == nil {
		t.Fatal("não achou a raiz 33333333")
	}

	if len(group.CNPJs) != 3 {
		t.Errorf("a raiz ficou com %d estabelecimentos, esperava 3", len(group.CNPJs))
	}

	if group.Name != "TRANSPORTES FICTICIOS LTDA" {
		t.Errorf("nome do grupo deu %q, esperava sem o sufixo de filial", group.Name)
	}
}

func TestLoadCompaniesSkipsBadRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientes.csv")
	content := "id;cnpj;nome\n1;11.111.111/0001-91;EMPRESA VALIDA LTDA\n2;123;CNPJ CURTO LTDA\n3\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	companies, err := loadCompanies(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(companies) != 1 {
		t.Errorf("deu %d raízes, esperava 1 - as duas linhas ruins deviam ser ignoradas", len(companies))
	}
}

func TestFindCertificatePrefersNewest(t *testing.T) {
	dir := t.TempDir()

	older := filepath.Join(dir, "EMPRESA EXEMPLO LTDA -- 11111111000191 -- senha antiga.pfx")
	newer := filepath.Join(dir, "EMPRESA EXEMPLO LTDA -- 11111111000191 -- senha nova.pfx")

	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}

	path, password, err := findCertificate(dir, "11111111")
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(path) != filepath.Base(newer) {
		t.Errorf("escolheu %q, esperava o mais recente", filepath.Base(path))
	}

	if password != "nova" {
		t.Errorf("senha deu %q, esperava %q", password, "nova")
	}
}
