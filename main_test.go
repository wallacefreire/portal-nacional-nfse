package main

import (
	"bytes"
	"errors"
	"fmt"
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

func TestExtractCompetence(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		want string
	}{
		{"competência anterior ao processamento",
			`<NFSe><dhProc>2026-10-03T09:00:00-03:00</dhProc><dCompet>2026-09-30</dCompet></NFSe>`,
			"2026-09"},
		{"sem dCompet", `<NFSe><dhProc>2026-07-29T16:01:25-03:00</dhProc></NFSe>`, "sem-data"},
		{"dCompet cortado", `<NFSe><dCompet>2026`, "sem-data"},
	}

	for _, c := range cases {
		got := extractCompetence([]byte(c.xml))
		if got != c.want {
			t.Errorf("%s: deu %q, esperava %q", c.name, got, c.want)
		}
	}
}

func TestServiceDirection(t *testing.T) {
	target := "11111111000191"
	prestada := "1100205" + "2" + "2" + target + "0000000000123" + "2610" + "123456789" + "0"
	tomada := "1100205" + "2" + "2" + "22222222000191" + "0000000000123" + "2610" + "123456789" + "0"

	cases := []struct {
		name string
		key  string
		want string
	}{
		{"prestador é o consultado", prestada, "prestadas"},
		{"prestador é outro", tomada, "tomadas"},
		{"chave curta", "123", "tomadas"},
	}

	for _, c := range cases {
		got := serviceDirection(c.key, target)
		if got != c.want {
			t.Errorf("%s: deu %q, esperava %q", c.name, got, c.want)
		}
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

func TestValidCompetence(t *testing.T) {
	valid := []string{"2026-10", "2025-01"}
	invalid := []string{"", "2026-1", "2026/10", "../2026", "2026-1a", "20261001"}

	for _, competence := range valid {
		if !validCompetence(competence) {
			t.Errorf("%q deveria ser válida", competence)
		}
	}
	for _, competence := range invalid {
		if validCompetence(competence) {
			t.Errorf("%q deveria ser inválida", competence)
		}
	}
}

func TestSummarizeMonthAndFolderToOpen(t *testing.T) {
	companyDir := t.TempDir()
	cnpj := "11111111000191"
	monthDir := filepath.Join(companyDir, cnpj, "2026-10")

	files := map[string]int{folderTaken: 3, folderProvided: 2}
	for folder, count := range files {
		dir := filepath.Join(monthDir, folder)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			name := filepath.Join(dir, fmt.Sprintf("nota%d.xml", i))
			if err := os.WriteFile(name, []byte("<NFSe/>"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	summary := summarizeMonth(companyDir, []string{cnpj, "11111111000272"}, "2026-10")
	if len(summary) != 2 {
		t.Fatalf("esperava 2 estabelecimentos, veio %d", len(summary))
	}
	if summary[0].Taken != 3 || summary[0].Provided != 2 || summary[0].Path != monthDir {
		t.Errorf("matriz: %+v", summary[0])
	}
	if summary[1].Taken != 0 || summary[1].Provided != 0 {
		t.Errorf("filial sem notas: %+v", summary[1])
	}

	cases := []struct {
		name       string
		cnpjs      []string
		competence string
		want       string
	}{
		{"um CNPJ com o mês baixado", []string{cnpj}, "2026-10", monthDir},
		{"mês sem notas", []string{cnpj}, "2026-09", companyDir},
		{"competência vazia", []string{cnpj}, "", companyDir},
		{"vários estabelecimentos", []string{cnpj, "11111111000272"}, "2026-10", companyDir},
	}
	for _, c := range cases {
		if got := folderToOpen(companyDir, c.cnpjs, c.competence); got != c.want {
			t.Errorf("%s: deu %q, esperava %q", c.name, got, c.want)
		}
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<NFSe/>"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRetireOldLayout(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 30, 0, 0, time.Local)
	cnpj := filepath.Join("EMPRESA TESTE_11111111", "11111111000191")

	t.Run("formato antigo é guardado e o controle zera junto", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "NFSE")
		config := Config{XMLBaseDir: base, StatePath: filepath.Join(base, "_controle", "nsu.json")}
		writeTestFile(t, filepath.Join(base, cnpj, "2026-10", "nota.xml"))
		writeTestFile(t, config.StatePath)

		retired, err := retireOldLayout(config, now)
		if err != nil {
			t.Fatal(err)
		}
		if retired != base+"-antigo-2026-11-09_083000" {
			t.Errorf("pasta guardada: %q", retired)
		}
		if _, err := os.Stat(filepath.Join(retired, cnpj, "2026-10", "nota.xml")); err != nil {
			t.Errorf("a nota antiga sumiu: %v", err)
		}
		if _, err := os.Stat(config.StatePath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("o ponteiro antigo continuou no lugar")
		}
		if _, err := os.Stat(filepath.Dir(config.StatePath)); err != nil {
			t.Errorf("a pasta de controle não foi recriada: %v", err)
		}
	})

	t.Run("formato novo fica intocado", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "NFSE")
		config := Config{XMLBaseDir: base, StatePath: filepath.Join(base, "_controle", "nsu.json")}
		writeTestFile(t, filepath.Join(base, cnpj, "2026-10", folderTaken, "nota.xml"))
		writeTestFile(t, filepath.Join(base, cnpj, "_eventos", "evento.xml"))
		writeTestFile(t, config.StatePath)

		retired, err := retireOldLayout(config, now)
		if err != nil || retired != "" {
			t.Fatalf("não devia mexer: %q, %v", retired, err)
		}
		if _, err := os.Stat(config.StatePath); err != nil {
			t.Errorf("o ponteiro sumiu: %v", err)
		}
	})

	t.Run("pasta vazia fica intocada", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "NFSE")
		os.MkdirAll(filepath.Join(base, "_controle"), 0o755)
		config := Config{XMLBaseDir: base, StatePath: filepath.Join(base, "_controle", "nsu.json")}

		if retired, err := retireOldLayout(config, now); err != nil || retired != "" {
			t.Fatalf("não devia mexer: %q, %v", retired, err)
		}
	})

	t.Run("controle fora da pasta das notas também zera", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "NFSE")
		config := Config{XMLBaseDir: base, StatePath: filepath.Join(dir, "controle", "nsu.json")}
		writeTestFile(t, filepath.Join(base, cnpj, "sem-data", "nota.xml"))
		writeTestFile(t, config.StatePath)

		if _, err := retireOldLayout(config, now); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(config.StatePath + ".antigo-2026-11-09_083000"); err != nil {
			t.Errorf("o ponteiro não foi guardado: %v", err)
		}
		if _, err := os.Stat(config.StatePath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("o ponteiro antigo continuou no lugar")
		}
	})
}

func TestFindCertificateAcceptsP12(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "EMPRESA EXEMPLO LTDA -- 11111111000191 -- SENHA abcd1234.p12")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OUTRA -- 11111111000191 -- senha x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, password, err := findCertificate(dir, "11111111")
	if err != nil {
		t.Fatal(err)
	}
	if found != path || password != "abcd1234" {
		t.Errorf("achou %q com senha %q", filepath.Base(found), password)
	}
}

func TestIsTemporaryBuild(t *testing.T) {
	cases := []struct {
		exe  string
		want bool
	}{
		{`C:\ContaComigo\NFSE\portalnacional.exe`, false},
		{`C:\Users\Fulano\AppData\Local\go-build\ab\cd\exe\portalnacional.exe`, true},
		{filepath.Join(os.TempDir(), "go-run123", "portalnacional.exe"), true},
	}
	for _, c := range cases {
		if got := isTemporaryBuild(c.exe); got != c.want {
			t.Errorf("%s: deu %v, esperava %v", c.exe, got, c.want)
		}
	}
}

func fakeExe(size int, fill byte) []byte {
	data := bytes.Repeat([]byte{fill}, size)
	copy(data, "MZ")
	return data
}

func TestApplyUpdate(t *testing.T) {
	setup := func(t *testing.T, current, incoming []byte) (string, string) {
		dir := t.TempDir()
		exe := filepath.Join(dir, "portalnacional.exe")
		source := filepath.Join(dir, "drive", "portalnacional.exe")
		writeTestFile(t, source)
		if err := os.WriteFile(exe, current, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, incoming, 0o644); err != nil {
			t.Fatal(err)
		}
		return exe, source
	}

	t.Run("versão igual não faz nada", func(t *testing.T) {
		same := fakeExe(minimumExeSize, 1)
		exe, source := setup(t, same, same)
		if updated, err := applyUpdate(source, exe); updated || err != nil {
			t.Fatalf("não devia atualizar: %v, %v", updated, err)
		}
	})

	t.Run("versão nova entra e a antiga fica guardada", func(t *testing.T) {
		old, fresh := fakeExe(minimumExeSize, 1), fakeExe(minimumExeSize, 2)
		exe, source := setup(t, old, fresh)

		updated, err := applyUpdate(source, exe)
		if !updated || err != nil {
			t.Fatalf("devia atualizar: %v, %v", updated, err)
		}
		if got, _ := os.ReadFile(exe); !bytes.Equal(got, fresh) {
			t.Error("o executável não foi trocado")
		}
		if got, _ := os.ReadFile(exe + ".antigo"); !bytes.Equal(got, old) {
			t.Error("a versão anterior não foi guardada")
		}
		if _, err := os.Stat(exe + ".novo"); !errors.Is(err, os.ErrNotExist) {
			t.Error("sobrou o arquivo temporário")
		}
	})

	t.Run("arquivo pela metade é recusado", func(t *testing.T) {
		old := fakeExe(minimumExeSize, 1)
		exe, source := setup(t, old, fakeExe(1000, 2))

		if updated, err := applyUpdate(source, exe); updated || err == nil {
			t.Fatalf("devia recusar: %v, %v", updated, err)
		}
		if got, _ := os.ReadFile(exe); !bytes.Equal(got, old) {
			t.Error("o executável atual foi mexido")
		}
	})

	t.Run("arquivo que não é executável é recusado", func(t *testing.T) {
		old := fakeExe(minimumExeSize, 1)
		exe, source := setup(t, old, bytes.Repeat([]byte{3}, minimumExeSize))

		if updated, err := applyUpdate(source, exe); updated || err == nil {
			t.Fatalf("devia recusar: %v, %v", updated, err)
		}
	})
}
