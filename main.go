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

	config, err = aplicarPadroes(config)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "--tela" {
		if err := servirWeb(config, "localhost:8080"); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--scan" {
		scanCertificates(config.CertificadosDir)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--cert" {
		raiz := os.Args[2]
		caminho, senha, err := findCertificate(config.CertificadosDir, raiz)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Certificado:", caminho)
		fmt.Println("Senha......:", senha)
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "--resetar" {
		alvo := os.Args[2]

		state, err := loadState(config.EstadoPath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		apagados := 0
		for cnpj, nsu := range state {
			if strings.HasPrefix(cnpj, alvo) {
				fmt.Printf(" %s estava no NSU %d\n", cnpj, nsu)
				delete(state, cnpj)
				apagados++
			}
		}

		if apagados == 0 {
			fmt.Printf("Nada encontrado no controle para %s\n", alvo)
			return
		}

		if err := saveState(config.EstadoPath, state); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\n%d ponteiro(s) apagado(s) - a proxima execucao baixa tudo de novo.\n", apagados)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--todas" {
		companies, err := loadCompanies(config.ClientesCSV)
		if err != nil {
			log.Fatal("Erro ao ler o CSV de clientes: ", err)
		}

		roots := make([]string, 0, len(companies))
		for root := range companies {
			roots = append(roots, root)
		}
		sort.Strings(roots)

		if len(os.Args) > 2 {
			limite, err := strconv.Atoi(os.Args[2])
			if err != nil || limite < 1 {
				log.Fatalf("Limite inválido: %q", os.Args[2])
			}
			if limite < len(roots) {
				roots = roots[:limite]
			}
		}

		state, err := loadState(config.EstadoPath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		var geral Resultado

		for i, root := range roots {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}

			fmt.Printf("\n[%d/%d]", i+1, len(roots))

			resultado, err := downloadRoot(config, state, root, companies[root].CNPJs)
			geral.Salvos += resultado.Salvos
			geral.Falhas += resultado.Falhas
			geral.Pendencias = append(geral.Pendencias, resultado.Pendencias...)

			if err != nil {
				log.Printf("PARANDO: %v", err)
				break
			}
		}

		fmt.Printf("\n===== RESUMO =====\n")
		fmt.Printf("Empresas processadas: %d\n", len(roots))
		fmt.Printf("Documentos salvos...: %d\n", geral.Salvos)
		fmt.Printf("Documentos com falha: %d\n", geral.Falhas)
		fmt.Printf("Empresas com erro...: %d\n", len(geral.Pendencias))

		for _, p := range geral.Pendencias {
			fmt.Printf("  [%s] %s %s - %s\n", p.Tipo, p.Raiz, p.Nome, p.Motivo)
		}
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "--empresa" {
		root := os.Args[2]

		var cnpjs []string
		if len(os.Args) > 3 {
			cnpjs = os.Args[3:]
		} else {
			companies, err := loadCompanies(config.ClientesCSV)
			if err != nil {
				log.Fatal("Erro ao ler o CSV de clientes: ", err)
			}

			company := companies[root]
			if company == nil {
				log.Fatalf("Raiz %s não está no CSV. Informe os CNPJs na mão :\n -- empresa %s <cnpj> <cnpj>", root, root)
			}
			cnpjs = company.CNPJs
		}

		state, err := loadState(config.EstadoPath)
		if err != nil {
			log.Fatal("Erro ao ler o arquivo de controle: ", err)
		}

		resultado, err := downloadRoot(config, state, root, cnpjs)
		if err != nil {
			log.Fatalf("Raiz %s: %v", root, err)
		}

		fmt.Printf("\nTotal da raiz %s: %d salvos, %d falhas\n", root, resultado.Salvos, resultado.Falhas)
		for _, p := range resultado.Pendencias {
			fmt.Printf("  PENDENTE [%s] %s\n", p.Tipo, p.Motivo)
		}
		return
	}

	certPath := os.Args[1]
	certPassword := os.Args[2]

	clientCertificate, companyCertificate, err := loadCertificate(certPath, certPassword, config.ConvertidosDir)
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

	statePath := config.EstadoPath
	state, err := loadState(statePath)
	if err != nil {
		log.Fatal("Erro ao ler o arquivo de controle", err)
	}

	if _, _, err := downloadCNPJ(httpClient, config, state, companyName, targetCNPJ); err != nil {
		log.Fatal("Erro ao baixar: ", err)
	}
}
