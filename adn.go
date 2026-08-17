package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ErrRateLimited sinaliza que o servidor pedir pra ir mais devagar
var ErrRateLimited = errors.New("limite de requisições atingido")

// DistributionResponse é a resposta inteira de GET /DFe/{NSU}
type DistributionResponse struct {
	StatusProcessamento string           `json:"statusProcessamento"`
	LoteDFe             []FiscalDocument `json:"loteDFe"`
}

// FiscalDocument é cada documento de dentro do lote
type FiscalDocument struct {
	NSU             int64  `json:"nsu"`
	ChaveAcesso     string `json:"chaveAcesso"`
	TipoDocumento   string `json:"tipoDocumento"`
	TipoEvento      string `json:"tipoEvento"`
	ArquivoXml      string `json:"arquivoXml"`
	DataHoraGeracao string `json:"dataHoraGeracao"`
}

func fetchBatchWithRetry(httpClient *http.Client, nsu int64, cnpjConsulta string) (*DistributionResponse, error) {
	waitTime := 5 * time.Second

	for attempt := 1; attempt <= 6; attempt++ {
		distribution, err := fetchBatch(httpClient, nsu, cnpjConsulta)
		if err == nil {
			return distribution, nil
		}

		if !errors.Is(err, ErrRateLimited) {
			return nil, err
		}

		log.Printf("429 no NSU %d - aguardando %s (tentativa %d/6)", nsu, waitTime, attempt)
		time.Sleep(waitTime)
		waitTime *= 2
	}

	return nil, fmt.Errorf("desisti no NSU %d após 6 tentativas", nsu)
}

func fetchBatch(httpClient *http.Client, nsu int64, cnpjConsulta string) (*DistributionResponse, error) {
	url := fmt.Sprintf("https://adn.nfse.gov.br/contribuintes/DFe/%d", nsu)
	if cnpjConsulta != "" {
		url += "?cnpjConsulta=" + cnpjConsulta
	}

	response, err := httpClient.Get(url)

	if err != nil {
		return nil, fmt.Errorf("Chamando a API no NSU %d: %w", nsu, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler a resposta do NSU %d: %w", nsu, err)
	}

	if response.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("servidor respondeu %s: %s", response.Status, resumir(body))
	}

	var distribution DistributionResponse
	if err := json.Unmarshal(body, &distribution); err != nil {
		return nil, fmt.Errorf("erro ao interpretar o JSON do NSU %d: %w", nsu, err)
	}

	return &distribution, nil
}

func unwrapXML(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar base64: %w", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewBuffer(compressed))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar leitor gzip: %w", err)
	}
	defer gzipReader.Close()

	xmlContent, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler conteúdo XML: %w", err)
	}

	return xmlContent, nil
}

func resumir(dados []byte) string {
	texto := strings.Join(strings.Fields(string(dados)), " ")
	if len(texto) > 200 {
		return texto[:200] + "..."
	}
	return texto
}
