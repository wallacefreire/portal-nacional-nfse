package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// ErrNoCertificate sinaliza que não existe .pfx para aquela raiz
var ErrNoCertificate = errors.New("nenhum certificado encontrado")

// ErrNoPasswordInName sinaliza que o .pfx existe mas a senha não está no nome do arquivo
var ErrNoPasswordInName = errors.New("senha não encontrada no nome do certificado")

// ErrInvalidCertificate sinaliza que o .pfx existe mas não abriu
var ErrInvalidCertificate = errors.New("não foi possível abrir o certificado")

func clientForRoot(config Config, root string) (*http.Client, *x509.Certificate, error) {
	certPath, certPassword, err := findCertificate(config.CertificatesDir, root)
	if err != nil {
		return nil, nil, err
	}

	clientCert, companyCert, err := loadCertificate(certPath, certPassword, config.ConvertedDir)
	if err != nil {
		return nil, nil, fmt.Errorf("%w da raiz %s (confira a senha no nome do arquivo)",
			ErrInvalidCertificate, root)
	}

	return newHTTPClient(clientCert), companyCert, nil
}

func findCertificate(certDir, root string) (string, string, error) {
	entries, err := os.ReadDir(certDir)
	if err != nil {
		return "", "", fmt.Errorf("erro ao ler a pasta de certificados: %w", err)
	}

	matches := 0
	chosen := ""
	chosenPassword := ""
	var chosenTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pfx") {
			continue
		}

		if !strings.Contains(onlyDigits(entry.Name()), root) {
			continue
		}

		matches++

		password, err := extractPasswordFromFilename(entry.Name())
		if err != nil {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if chosen == "" || info.ModTime().After(chosenTime) {
			chosen, chosenPassword, chosenTime = entry.Name(), password, info.ModTime()
		}
	}

	if matches == 0 {
		return "", "", fmt.Errorf("%w para a raiz %s", ErrNoCertificate, root)
	}

	if chosen == "" {
		return "", "", fmt.Errorf("%w para a raiz %s", ErrNoPasswordInName, root)
	}

	if matches > 1 {
		log.Printf("%d certificados para a raiz %s - usando o mais recente (%s)", matches, root, chosen)
	}

	return filepath.Join(certDir, chosen), chosenPassword, nil
}

func loadCertificate(originalPath, password, cacheDir string) (tls.Certificate, *x509.Certificate, error) {
	cert, leaf, err := tryLoadPFX(originalPath, password)
	if err == nil {
		return cert, leaf, nil
	}

	cachePath := filepath.Join(cacheDir, filepath.Base(originalPath))
	if isNewer(cachePath, originalPath) {
		if cert, leaf, err := tryLoadPFX(cachePath, password); err == nil {
			return cert, leaf, nil
		}
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("criando pasta de cache: %w", err)
	}
	if err := convertWithWindows(originalPath, cachePath, password); err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("convertendo o certificado: %w", err)
	}

	return tryLoadPFX(cachePath, password)
}

func tryLoadPFX(path, password string) (tls.Certificate, *x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	privateKey, leaf, authorities, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{leaf.Raw},
		PrivateKey:  privateKey,
	}
	for _, authority := range authorities {
		cert.Certificate = append(cert.Certificate, authority.Raw)
	}

	return cert, leaf, nil
}

func isNewer(cachePath, originalPath string) bool {
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		return false
	}
	originalInfo, err := os.Stat(originalPath)
	if err != nil {
		return false
	}
	return !cacheInfo.ModTime().Before(originalInfo.ModTime())
}

func convertWithWindows(originalPath, cachePath, password string) error {
	script := fmt.Sprintf(
		`$p = ConvertTo-SecureString %q -AsPlainText -Force; `+
			`$c = Import-PfxCertificate -FilePath %q -CertStoreLocation Cert:\CurrentUser\My -Password $p -Exportable; `+
			`Export-PfxCertificate -Cert $c -FilePath %q -Password $p | Out-Null; `+
			`Remove-Item ("Cert:\CurrentUser\My\" + $c.Thumbprint)`,
		password, originalPath, cachePath,
	)

	cmd := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("powershell falhou: %v: %s", err, string(output))
	}
	return nil
}

func extractPasswordFromFilename(filename string) (string, error) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	if position := strings.LastIndex(strings.ToLower(name), "senha"); position >= 0 {
		if password := strings.Trim(name[position+len("senha"):], " -_."); password != "" {
			return password, nil
		}
	}

	for _, separator := range []string{" -- ", " - "} {
		if position := strings.LastIndex(name, separator); position >= 0 {
			if password := strings.Trim(name[position+len(separator):], " -_."); password != "" {
				return password, nil
			}
		}
	}

	return "", fmt.Errorf("nenhuma senha encontrada no nome do arquivo")
}

func newHTTPClient(clientCert tls.Certificate) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				MinVersion:   tls.VersionTLS12,
			},
		},
		Timeout: 30 * time.Second,
	}
}

func scanCertificates(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal("Erro ao ler a pasta de certificados: ", err)
	}

	okCount := 0
	failCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pfx") {
			continue
		}

		password, err := extractPasswordFromFilename(entry.Name())
		if err != nil {
			fmt.Printf("[FALHOU] %s\n motivo: %v\n", entry.Name(), err)
			failCount++
			continue
		}

		fmt.Printf("[ok] %-70s senha: %s\n", entry.Name(), password)
		okCount++
	}

	fmt.Printf("\nTotal: %d lidos, %d sem senha no nome\n", okCount, failCount)
}
