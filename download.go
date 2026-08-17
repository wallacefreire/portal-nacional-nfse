package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const pausaEntreLotes = 1 * time.Second

type Pendencia struct {
	Raiz   string
	CNPJ   string
	Nome   string
	Tipo   string
	Motivo string
}

type Resultado struct {
	Salvos     int
	Falhas     int
	Pasta      string
	Pendencias []Pendencia
}

const (
	TipoSemCertificado      = "Sem certificado"
	TipoVencido             = "Certificado Vencido"
	TipoFalhaConsulta       = "Falha na Consulta"
	TipoSenhaNoNome         = "Senha não cadastrada"
	TipoCertificadoInvalido = "Certificado não abre"
)

func downloadRoot(config Config, state NSUState, root string, cnpjs []string) (Resultado, error) {
	var resultado Resultado

	httpClient, companyCert, err := clientForRoot(config, root)
	if err != nil {
		tipo := TipoFalhaConsulta
		switch {
		case errors.Is(err, ErrSemCertificado):
			tipo = TipoSemCertificado
		case errors.Is(err, ErrSenhaNoNome):
			tipo = TipoSenhaNoNome
		case errors.Is(err, ErrCertificadoInvalido):
			tipo = TipoCertificadoInvalido
		}

		resultado.Pendencias = append(resultado.Pendencias, Pendencia{
			Raiz:   root,
			Tipo:   tipo,
			Motivo: err.Error(),
		})
		return resultado, nil
	}

	commonNameParts := strings.Split(companyCert.Subject.CommonName, ":")
	companyName := strings.TrimSpace(commonNameParts[0])

	safeName := strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(companyName)
	resultado.Pasta = filepath.Join(config.XMLBaseDir, safeName+"_"+root)

	fmt.Printf("\n=== %s (raiz %s) - validade %s ===\n",
		companyName, root, companyCert.NotAfter.Format("02/01/2006"))

	if time.Now().After(companyCert.NotAfter) {
		fmt.Println("ATENÇÃO: certificado vencido - pulando empresa")

		resultado.Pendencias = append(resultado.Pendencias, Pendencia{
			Raiz:   root,
			Nome:   companyName,
			Tipo:   TipoVencido,
			Motivo: "vencido em " + companyCert.NotAfter.Format("02/01/2006"),
		})
		return resultado, nil
	}

	for i, cnpj := range cnpjs {
		if i > 0 {
			time.Sleep(pausaEntreLotes)
		}

		fmt.Println("Consultando:", cnpj)

		saved, failed, err := downloadCNPJ(httpClient, config, state, companyName, cnpj)
		resultado.Salvos += saved
		resultado.Falhas += failed

		if err != nil {
			if errors.Is(err, ErrEstadoNaoSalvo) {
				return resultado, err
			}

			log.Printf("raiz %s, CNPJ %s: %v", root, cnpj, err)
			resultado.Pendencias = append(resultado.Pendencias, Pendencia{
				Raiz:   root,
				CNPJ:   cnpj,
				Nome:   companyName,
				Tipo:   TipoFalhaConsulta,
				Motivo: err.Error(),
			})
		}
	}

	return resultado, nil
}

func downloadCNPJ(httpClient *http.Client, config Config, state NSUState, companyName, targetCNPJ string) (int, int, error) {
	safeName := strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(companyName)

	root := targetCNPJ
	if len(targetCNPJ) >= 8 {
		root = targetCNPJ[:8]
	}
	groupFolder := safeName + "_" + root

	cnpjConsulta := targetCNPJ

	savedCount := 0
	failedCount := 0
	batchCount := 0
	currentNSU := state[targetCNPJ]
	pastasCriadas := map[string]bool{}

	if currentNSU > 0 {
		fmt.Printf("Retomando do NSU %d\n", currentNSU)
	} else {
		fmt.Println("Primeira execução - baixando desde o início")
	}

	for {
		inicioBusca := time.Now()
		distribution, err := fetchBatchWithRetry(httpClient, currentNSU, cnpjConsulta)
		duracaoBusca := time.Since(inicioBusca)
		if err != nil {
			return savedCount, failedCount, fmt.Errorf("Erro ao buscar lote no NSU %d: %w", currentNSU, err)
		}

		if distribution.StatusProcessamento == "NENHUM_DOCUMENTO_LOCALIZADO" {
			fmt.Println("Nenhum documento localizado.")
			break
		}

		if len(distribution.LoteDFe) == 0 {
			fmt.Println("Lote vazio - encerrando.")
			break
		}

		batchCount++
		fmt.Printf("Lote %d: %d documentos a partir do NSU %d (busca %.1fs)\n",
			batchCount, len(distribution.LoteDFe), currentNSU, duracaoBusca.Seconds())

		var highestNSU int64
		falhasDeGravacao := 0

		for _, document := range distribution.LoteDFe {
			if document.NSU > highestNSU {
				highestNSU = document.NSU
			}

			xmlContent, err := unwrapXML(document.ArquivoXml)
			if err != nil {
				log.Printf("Erro ao descompactar XML do NSU %d: %v", document.NSU, err)
				failedCount++
				continue
			}

			issueDate := extractIssueDate(xmlContent)

			var outputDir string
			if document.TipoDocumento == "EVENTO" {
				outputDir = filepath.Join(config.XMLBaseDir, groupFolder, targetCNPJ, "_eventos")
			} else {
				outputDir = filepath.Join(config.XMLBaseDir, groupFolder, targetCNPJ, issueDate)
			}

			filename := document.ChaveAcesso + ".xml"
			if document.TipoDocumento == "EVENTO" {
				filename = fmt.Sprintf("%s-%d.xml", document.ChaveAcesso, document.NSU)
			}

			fullPath := filepath.Join(outputDir, filename)
			if err := gravarComTentativas(pastasCriadas, outputDir, fullPath, xmlContent); err != nil {
				log.Printf("Erro ao gravar NSU %d: %v", document.NSU, err)
				failedCount++
				falhasDeGravacao++
				continue
			}

			savedCount++

		}

		if falhasDeGravacao > 0 {
			return savedCount, failedCount, fmt.Errorf(
				"%d de %d documentos do lote não foram gravados - ponteiro mantido em %d",
				falhasDeGravacao, len(distribution.LoteDFe), currentNSU)
		}

		if highestNSU <= currentNSU {
			fmt.Println("NSU não avançou - encerrando por segurança.")
			break
		}

		currentNSU = highestNSU
		state[targetCNPJ] = currentNSU

		if err := saveState(config.EstadoPath, state); err != nil {
			return savedCount, failedCount, fmt.Errorf("%w (NSU %d): %v", ErrEstadoNaoSalvo, currentNSU, err)
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Printf("\nLotes: %d, Documentos salvos: %d, Falhas: %d | Último NSU: %d\n",
		batchCount, savedCount, failedCount, currentNSU)

	return savedCount, failedCount, nil
}

func extractIssueDate(xmlContent []byte) string {
	texto := string(xmlContent)

	inicio := strings.Index(texto, "<dhProc>")
	if inicio < 0 {
		return "sem-data"
	}
	inicio += len("<dhProc>")

	if len(texto) < inicio+7 {
		return "sem-data"
	}

	ano := texto[inicio : inicio+4]
	mes := texto[inicio+5 : inicio+7]

	return ano + "-" + mes
}

func gravarComTentativas(pastasCriadas map[string]bool, dir, caminho string, conteudo []byte) error {
	var err error

	for tentativa := 1; tentativa <= 3; tentativa++ {
		if tentativa > 1 {
			time.Sleep(time.Duration(tentativa-1) * time.Second)
		}

		if !pastasCriadas[dir] {
			if err = os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
			pastasCriadas[dir] = true
		}

		if err = os.WriteFile(caminho, conteudo, 0o644); err == nil {
			return nil
		}

		delete(pastasCriadas, dir)
	}

	return err
}
