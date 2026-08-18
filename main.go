package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatal("Erro ao carregar a configuração: ", err)
	}

	config, err = applyDefaults(config)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 1 || os.Args[1] == "--tela" {
		if err := serveWeb(config, "localhost:8080"); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--scan" {
		scanCertificates(config.CertificatesDir)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--cert" {
		root := os.Args[2]
		path, password, err := findCertificate(config.CertificatesDir, root)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Certificado:", path)
		fmt.Println("Senha......:", password)
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "--resetar" {
		target := os.Args[2]

		state, err := loadState(config.StatePath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		deleted := 0
		for cnpj, nsu := range state {
			if strings.HasPrefix(cnpj, target) {
				fmt.Printf(" %s estava no NSU %d\n", cnpj, nsu)
				delete(state, cnpj)
				deleted++
			}
		}

		if deleted == 0 {
			fmt.Printf("Nada encontrado no controle para %s\n", target)
			return
		}

		if err := saveState(config.StatePath, state); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\n%d ponteiro(s) apagado(s) - a proxima execucao baixa tudo de novo.\n", deleted)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--todas" {
		companies, err := loadCompanies(config.ClientsCSV)
		if err != nil {
			log.Fatal("Erro ao ler o CSV de clientes: ", err)
		}

		roots := make([]string, 0, len(companies))
		for root := range companies {
			roots = append(roots, root)
		}
		sort.Strings(roots)

		if len(os.Args) > 2 {
			limit, err := strconv.Atoi(os.Args[2])
			if err != nil || limit < 1 {
				log.Fatalf("Limite inválido: %q", os.Args[2])
			}
			if limit < len(roots) {
				roots = roots[:limit]
			}
		}

		state, err := loadState(config.StatePath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		var total Result

		for i, root := range roots {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}

			fmt.Printf("\n[%d/%d]", i+1, len(roots))

			result, err := downloadRoot(config, state, root, companies[root].CNPJs)
			total.Saved += result.Saved
			total.Failed += result.Failed
			total.Issues = append(total.Issues, result.Issues...)

			if err != nil {
				log.Printf("PARANDO: %v", err)
				break
			}
		}

		fmt.Printf("\n===== RESUMO =====\n")
		fmt.Printf("Empresas processadas: %d\n", len(roots))
		fmt.Printf("Documentos salvos...: %d\n", total.Saved)
		fmt.Printf("Documentos com falha: %d\n", total.Failed)
		fmt.Printf("Empresas com erro...: %d\n", len(total.Issues))

		for _, issue := range total.Issues {
			fmt.Printf("  [%s] %s %s - %s\n", issue.Kind, issue.Root, issue.Name, issue.Reason)
		}
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "--empresa" {
		root := os.Args[2]

		var cnpjs []string
		if len(os.Args) > 3 {
			cnpjs = os.Args[3:]
		} else {
			companies, err := loadCompanies(config.ClientsCSV)
			if err != nil {
				log.Fatal("Erro ao ler o CSV de clientes: ", err)
			}

			company := companies[root]
			if company == nil {
				log.Fatalf("Raiz %s não está no CSV. Informe os CNPJs na mão :\n -- empresa %s <cnpj> <cnpj>", root, root)
			}
			cnpjs = company.CNPJs
		}

		state, err := loadState(config.StatePath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		result, err := downloadRoot(config, state, root, cnpjs)
		if err != nil {
			log.Fatalf("Raiz %s: %v", root, err)
		}

		fmt.Printf("\nTotal da raiz %s: %d salvos, %d falhas\n", root, result.Saved, result.Failed)
		for _, issue := range result.Issues {
			fmt.Printf("  PENDENTE [%s] %s\n", issue.Kind, issue.Reason)
		}
		return
	}

	certPath := os.Args[1]
	certPassword := os.Args[2]

	clientCertificate, companyCertificate, err := loadCertificate(certPath, certPassword, config.ConvertedDir)
	if err != nil {
		log.Fatal("Erro ao carregar o certificado: ", err)
	}

	fmt.Println("Empresa..:", companyCertificate.Subject.CommonName)
	fmt.Println("Validade.:", companyCertificate.NotAfter.Format("02/01/2006"))

	if time.Now().After(companyCertificate.NotAfter) {
		fmt.Println("Certificado expirado!")
	}

	httpClient := newHTTPClient(clientCertificate)
	commonNameParts := strings.Split(companyCertificate.Subject.CommonName, ":")
	companyCNPJ := commonNameParts[len(commonNameParts)-1]

	targetCNPJ := companyCNPJ
	if len(os.Args) > 3 {
		targetCNPJ = os.Args[3]
	}

	fmt.Println("Consultando:", targetCNPJ)

	companyName := strings.TrimSpace(commonNameParts[0])

	statePath := config.StatePath
	state, err := loadState(statePath)
	if err != nil {
		log.Fatal("Erro ao ler o arquivo de controle", err)
	}

	if _, _, err := downloadCNPJ(httpClient, config, state, companyName, targetCNPJ); err != nil {
		log.Fatal("Erro ao baixar: ", err)
	}
}
