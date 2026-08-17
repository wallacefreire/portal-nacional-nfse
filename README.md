# Portal Nacional NFS-e

Baixa automaticamente os XMLs de NFS-e (Nota Fiscal de Serviço eletrônica) a partir do **Ambiente de Dados Nacional (ADN)**, autenticando com certificado digital A1.

Escrito em Go, sem dependências além da biblioteca de leitura de certificados.

---

## O problema

Um escritório contábil precisa importar, todo início de mês, as NFS-e de centenas de empresas. O sistema que eles já usam resolve as **matrizes**, mas não consegue baixar as notas das **filiais** — e fazer isso à mão, empresa por empresa, pelo portal web (com captcha), é inviável.

Este programa preenche exatamente essa lacuna: baixa os XMLs das filiais (e de qualquer CNPJ) direto da API oficial, sem intervenção manual, e deixa os arquivos prontos para importação.

---

## Como funciona

O ADN não funciona como uma consulta ("me dê as notas de julho"). Ele funciona como uma **caixa postal numerada**: cada documento fiscal destinado a um CNPJ recebe um número sequencial (NSU), e a única operação é *"me dê o que chegou depois do NSU X"*.

```
1. Abre o certificado .pfx (convertendo o formato se necessário)
2. Autentica no ADN por TLS mútuo (o certificado É a credencial)
3. Pede lotes a partir do último NSU lido      ── repete até acabar
4. Cada documento vem compactado (gzip) e codificado (base64)
5. Desembrulha e grava o XML em disco
6. Salva o último NSU, para a próxima execução ser incremental
```

A primeira execução de uma empresa baixa todo o histórico; as seguintes baixam apenas o que chegou desde a última vez.

---

## Detalhes técnicos que valeram o esforço

**Certificados em BER que o Go não lê.** Certificados ICP-Brasil antigos são codificados em BER (com comprimento indefinido), e a biblioteca padrão do Go só aceita DER. A tentativa de converter em Go puro falhou — esses certificados usam camadas aninhadas de BER que um conversor genérico corrompe. A solução foi reexportar o certificado via API de criptografia do Windows (que reencripta em DER), guardando o resultado num cache compartilhado para que a conversão aconteça uma única vez.

**Baixar filiais com o certificado da matriz.** O e-CNPJ da matriz representa a pessoa jurídica inteira. A API aceita um parâmetro opcional (`cnpjConsulta`) que permite, autenticado como matriz, pedir a caixa postal de uma filial — cada estabelecimento com sua própria numeração de NSU independente.

**Rate limiting agressivo.** O ADN devolve HTTP 429 com poucas requisições em sequência. O cliente trata isso com _backoff_ exponencial (espera dobrando a cada tentativa) e uma pausa entre lotes.

**Escrita atômica do estado.** O ponteiro de NSU é gravado num arquivo temporário e renomeado por cima do definitivo — se o programa for interrompido no meio, o arquivo de controle nunca fica corrompido.

---

## Requisitos

- Go 1.21+
- Certificado digital **A1** (arquivo `.pfx` ou `.p12`)
- Windows com **PowerShell 7 (`pwsh`)** — usado apenas para reconverter certificados no formato antigo (BER)

---

## Configuração

Copie o arquivo de exemplo e preencha com os seus caminhos:

```bash
cp config.exemplo.json config.json
```

```json
{
  "certificadosDir": "C:\\caminho\\para\\certificados",
  "convertidosDir":  "C:\\caminho\\para\\convertidos",
  "xmlBaseDir":      "C:\\caminho\\para\\xmls",
  "estadoPath":      "C:\\caminho\\para\\nsu.json"
}
```

O `config.json` fica fora do controle de versão (contém caminhos internos).

---

## Uso

Baixar as notas de um CNPJ usando seu próprio certificado:

```bash
go run . "caminho/certificado.pfx" "senha"
```

Baixar as notas de uma **filial**, usando o certificado da matriz:

```bash
go run . "caminho/certificado-matriz.pfx" "senha" 00000000000200
```

Conferir a extração de senha dos nomes dos arquivos, sem baixar nada:

```bash
go run . --scan
```

Os XMLs são salvos organizados por empresa:

```
xmls/
└── RAZAO SOCIAL_00000000000100/
    ├── NFSE/     ← notas fiscais
    └── EVENTO/   ← cancelamentos e demais eventos
```

---

## Limitações

- A API não filtra por data — a filtragem por competência é feita localmente, sobre os XMLs já baixados.
- A reconversão de certificados BER depende do Windows; certificados já em DER funcionam em qualquer sistema.
- O CNPJ das filiais precisa ser informado (a descoberta automática de estabelecimentos ainda não é feita).

---

## Stack

Go · TLS mútuo (mTLS) · PKCS#12 · REST · gzip/base64

---

## Licença

MIT
