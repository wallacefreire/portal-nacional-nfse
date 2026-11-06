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

const pauseBetweenBatches = 1 * time.Second

type Issue struct {
	Root   string
	CNPJ   string
	Name   string
	Kind   string
	Reason string
}

type Result struct {
	Saved  int
	Failed int
	Folder string
	Issues []Issue
}

const (
	IssueNoCertificate  = "Sem certificado"
	IssueExpired        = "Certificado vencido"
	IssueQueryFailed    = "Falha na consulta"
	IssueNoPassword     = "Senha não cadastrada"
	IssueBadCertificate = "Certificado não abre"
)

func downloadRoot(config Config, state NSUState, root string, cnpjs []string, task *Task) (Result, error) {
	var result Result

	httpClient, companyCert, err := clientForRoot(config, root)
	if err != nil {
		kind := IssueQueryFailed
		switch {
		case errors.Is(err, ErrNoCertificate):
			kind = IssueNoCertificate
		case errors.Is(err, ErrNoPasswordInName):
			kind = IssueNoPassword
		case errors.Is(err, ErrInvalidCertificate):
			kind = IssueBadCertificate
		}

		result.Issues = append(result.Issues, Issue{
			Root:   root,
			Kind:   kind,
			Reason: err.Error(),
		})
		return result, nil
	}

	commonNameParts := strings.Split(companyCert.Subject.CommonName, ":")
	companyName := strings.TrimSpace(commonNameParts[0])

	result.Folder = companyFolder(config, companyName, root)

	log.Printf("=== %s (raiz %s) - validade %s ===",
		companyName, root, companyCert.NotAfter.Format("02/01/2006"))

	if time.Now().After(companyCert.NotAfter) {
		log.Println("ATENÇÃO: certificado vencido - pulando empresa")

		result.Issues = append(result.Issues, Issue{
			Root:   root,
			Name:   companyName,
			Kind:   IssueExpired,
			Reason: "vencido em " + companyCert.NotAfter.Format("02/01/2006"),
		})
		return result, nil
	}

	for i, cnpj := range cnpjs {
		if i > 0 {
			time.Sleep(pauseBetweenBatches)
		}

		log.Println("Consultando:", cnpj)
		task.startStep(i+1, len(cnpjs))

		saved, failed, err := downloadCNPJ(httpClient, config, state, companyName, cnpj, task)

		result.Saved += saved
		result.Failed += failed

		if err != nil {
			if errors.Is(err, ErrStateNotSaved) {
				return result, err
			}

			log.Printf("raiz %s, CNPJ %s: %v", root, cnpj, err)
			result.Issues = append(result.Issues, Issue{
				Root:   root,
				CNPJ:   cnpj,
				Name:   companyName,
				Kind:   IssueQueryFailed,
				Reason: err.Error(),
			})
		}
	}

	return result, nil
}

func downloadCNPJ(httpClient *http.Client, config Config, state NSUState, companyName, targetCNPJ string, task *Task) (int, int, error) {
	root := targetCNPJ
	if len(targetCNPJ) >= 8 {
		root = targetCNPJ[:8]
	}
	cnpjFolder := filepath.Join(companyFolder(config, companyName, root), targetCNPJ)

	cnpjConsulta := targetCNPJ

	savedCount := 0
	failedCount := 0
	batchCount := 0
	currentNSU := state[targetCNPJ]
	createdDirs := map[string]bool{}

	if currentNSU > 0 {
		log.Printf("Retomando do NSU %d", currentNSU)
	} else {
		log.Println("Primeira execução - baixando desde o início")
	}

	for {
		fetchStart := time.Now()
		distribution, err := fetchBatchWithRetry(httpClient, currentNSU, cnpjConsulta)
		fetchDuration := time.Since(fetchStart)
		if err != nil {
			return savedCount, failedCount, fmt.Errorf("erro ao buscar lote no NSU %d: %w", currentNSU, err)
		}

		if distribution.StatusProcessamento == "NENHUM_DOCUMENTO_LOCALIZADO" {
			log.Println("Nenhum documento localizado.")
			break
		}

		if len(distribution.LoteDFe) == 0 {
			log.Println("Lote vazio - encerrando.")
			break
		}

		batchCount++
		log.Printf("Lote %d: %d documentos a partir do NSU %d (busca %.1fs)",
			batchCount, len(distribution.LoteDFe), currentNSU, fetchDuration.Seconds())

		savedBefore, failedBefore := savedCount, failedCount

		var highestNSU int64
		writeFailures := 0

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

			var outputDir string
			if document.TipoDocumento == "EVENTO" {
				outputDir = filepath.Join(cnpjFolder, "_eventos")
			} else {
				outputDir = filepath.Join(cnpjFolder,
					extractCompetence(xmlContent),
					serviceDirection(document.ChaveAcesso, targetCNPJ))
			}

			filename := document.ChaveAcesso + ".xml"
			if document.TipoDocumento == "EVENTO" {
				filename = fmt.Sprintf("%s-%d.xml", document.ChaveAcesso, document.NSU)
			}

			fullPath := filepath.Join(outputDir, filename)
			if err := writeWithRetries(createdDirs, outputDir, fullPath, xmlContent); err != nil {
				log.Printf("Erro ao gravar NSU %d: %v", document.NSU, err)
				failedCount++
				writeFailures++
				continue
			}

			savedCount++

		}

		if writeFailures > 0 {
			return savedCount, failedCount, fmt.Errorf(
				"%d de %d documentos do lote não foram gravados - ponteiro mantido em %d",
				writeFailures, len(distribution.LoteDFe), currentNSU)
		}

		if highestNSU <= currentNSU {
			log.Println("NSU não avançou - encerrando por segurança.")
			break
		}

		currentNSU = highestNSU
		state[targetCNPJ] = currentNSU

		if err := saveState(config.StatePath, state); err != nil {
			return savedCount, failedCount, fmt.Errorf("%w (NSU %d): %v", ErrStateNotSaved, currentNSU, err)
		}

		task.progress(savedCount-savedBefore, failedCount-failedBefore)

		time.Sleep(pauseBetweenBatches)
	}

	log.Printf("Lotes: %d, Documentos salvos: %d, Falhas: %d | Último NSU: %d",
		batchCount, savedCount, failedCount, currentNSU)

	return savedCount, failedCount, nil
}

func extractCompetence(xmlContent []byte) string {
	text := string(xmlContent)

	start := strings.Index(text, "<dCompet>")
	if start < 0 {
		return "sem-data"
	}
	start += len("<dCompet>")

	if len(text) < start+7 {
		return "sem-data"
	}

	year := text[start : start+4]
	month := text[start+5 : start+7]

	return year + "-" + month
}

const (
	folderTaken    = "tomadas"
	folderProvided = "prestadas"
)

func serviceDirection(accessKey, targetCNPJ string) string {
	if len(accessKey) >= 23 && accessKey[9:23] == targetCNPJ {
		return folderProvided
	}
	return folderTaken
}

type MonthFolder struct {
	CNPJ     string `json:"cnpj"`
	Path     string `json:"caminho"`
	Taken    int    `json:"tomadas"`
	Provided int    `json:"prestadas"`
}

func summarizeMonth(companyDir string, cnpjs []string, competence string) []MonthFolder {
	summary := make([]MonthFolder, 0, len(cnpjs))
	for _, cnpj := range cnpjs {
		monthDir := filepath.Join(companyDir, cnpj, competence)
		summary = append(summary, MonthFolder{
			CNPJ:     cnpj,
			Path:     monthDir,
			Taken:    countXML(filepath.Join(monthDir, folderTaken)),
			Provided: countXML(filepath.Join(monthDir, folderProvided)),
		})
	}
	return summary
}

func countXML(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".xml") {
			count++
		}
	}
	return count
}

func validCompetence(competence string) bool {
	if len(competence) != 7 || competence[4] != '-' {
		return false
	}
	for i, char := range competence {
		if i != 4 && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func companyFolder(config Config, companyName, root string) string {
	safeName := strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(companyName)
	return filepath.Join(config.XMLBaseDir, safeName+"_"+root)
}

func writeWithRetries(createdDirs map[string]bool, dir, path string, content []byte) error {
	var err error

	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * time.Second)
		}

		if !createdDirs[dir] {
			if err = os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
			createdDirs[dir] = true
		}

		if err = os.WriteFile(path, content, 0o644); err == nil {
			return nil
		}

		delete(createdDirs, dir)
	}

	return err
}
