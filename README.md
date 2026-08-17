# Portal Nacional NFS-e

Baixa automaticamente os XMLs de NFS-e (Nota Fiscal de Serviço eletrônica) a partir do **Ambiente de Dados Nacional (ADN)**, autenticando com certificado digital A1.

Escrito em Go, sem dependências além da biblioteca de leitura de certificados — inclusive a interface web, que usa só a biblioteca padrão e é compilada dentro do executável.

---

## O problema

Um escritório contábil precisa importar, todo início de mês, as NFS-e de centenas de empresas. O sistema que eles já usam resolve as **matrizes**, mas não baixa as notas das **filiais** — nem as notas **emitidas**, já que ele importa apenas o que a empresa recebeu. Fazer isso à mão, empresa por empresa, pelo portal web (com captcha), é inviável.

Este programa preenche exatamente essa lacuna: baixa os XMLs direto da API oficial, sem intervenção manual, e deixa os arquivos prontos para importação.

---

## A interface

![Tela de download de NFS-e](docs-tela.png)

Quem opera no dia a dia são auxiliares contábeis, não desenvolvedores. Então o alvo final não é a linha de comando: é uma tela.

```
go run . --tela
```

Sobe um servidor local, sem instalar nada. O auxiliar abre o navegador, busca a empresa pelo nome ou CNPJ, clica, e acompanha o resultado — quantas notas vieram e em que pasta foram salvas.

A lista sai de um CSV de clientes, agrupada por raiz de CNPJ, de forma que **um clique baixa a matriz e todas as filiais** daquela empresa.

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

A primeira execução de uma empresa baixa todo o histórico; as seguintes baixam apenas o que chegou desde a última vez. Na prática: uma empresa com anos de notas leva minutos na estreia e segundos todo mês seguinte.

---

## Detalhes técnicos que valeram o esforço

**Certificados em BER que o Go não lê.** Certificados ICP-Brasil antigos são codificados em BER (com comprimento indefinido), e a biblioteca padrão do Go só aceita DER. A tentativa de converter em Go puro falhou — esses certificados usam camadas aninhadas de BER que um conversor genérico corrompe. A solução foi reexportar o certificado via API de criptografia do Windows (que reencripta em DER), guardando o resultado num cache compartilhado para que a conversão aconteça uma única vez.

**Baixar filiais com o certificado da matriz.** O e-CNPJ da matriz representa a pessoa jurídica inteira. A API aceita um parâmetro opcional (`cnpjConsulta`) que permite, autenticado como matriz, pedir a caixa postal de uma filial — cada estabelecimento com sua própria numeração de NSU independente.

**O download não pode responder na mesma requisição.** A primeira carga de uma empresa grande leva minutos, e o navegador desiste antes. A rota de download dispara uma *goroutine* e responde imediatamente; a página consulta o estado a cada dois segundos até terminar. O estado vive num mapa protegido por mutex, e a leitura devolve cópia — nunca o ponteiro que a goroutine está escrevendo.

**A pausa entre requisições foi medida, não estimada.** O ADN devolve HTTP 429 com poucas requisições em sequência, e a primeira versão usava dois segundos de pausa por precaução. Instrumentando o tempo real de cada busca (0,2 a 0,4 s), ficou claro que a espera era 85% do tempo de execução. Um segundo se mostrou suficiente — 13 lotes seguidos sem nenhum 429 — e cortou o tempo pela metade. Abaixo disso o ganho não paga o risco: cada 429 custa cinco segundos de *backoff*.

**"Gravou" não significa "chegou".** Uma versão anterior escrevia direto numa pasta sincronizada em nuvem. O programa reportou 12.456 arquivos salvos e zero falhas — e nada havia chegado ao destino: o sistema operacional aceitava a escrita e o cliente de sincronização falhava depois, em silêncio. O mesmo código, apontado para disco local, gravou 12.476 de 12.476. A lição virou regra do projeto: só se pode afirmar o que foi verificado onde importa.

**Escrita atômica do estado.** O ponteiro de NSU é gravado num arquivo temporário e renomeado por cima do definitivo — se o programa for interrompido no meio, o arquivo de controle nunca fica corrompido. E o ponteiro só avança quando o lote inteiro foi gravado: na dúvida entre repetir e pular um documento, o projeto repete, porque arquivo repetido é sobrescrito e nota pulada some para sempre.

---

## Requisitos

- Go 1.21+
- Certificado digital **A1** (arquivo `.pfx` ou `.p12`)
- **PowerShell 7 (`pwsh`)** apenas para reconverter certificados no formato BER antigo; certificados já em DER, ou já convertidos no cache, dispensam

---

## Configuração

Copie o arquivo de exemplo:

```bash
cp config.exemplo.json config.json
```

```json
{
  "certificadosDir": "C:\\caminho\\para\\certificados",
  "convertidosDir":  "C:\\caminho\\para\\convertidos",
  "clientesCSV":     "C:\\caminho\\para\\clientes.csv",
  "xmlBaseDir":      "",
  "estadoPath":      ""
}
```

Campos deixados **vazios** assumem os padrões e criam as pastas sozinhos — `~/Documents/NFSE` para os XMLs e `~/Documents/NFSE/_controle/nsu.json` para o ponteiro. É o que permite rodar numa máquina nova sem configurar nada.

O `config.json` fica fora do controle de versão, por conter caminhos internos.

---

## Uso

```bash
go run . --tela                          # interface web em http://localhost:8080
go run . --empresa <raiz8> [cnpj14...]   # uma empresa; sem CNPJ, lê os do CSV
go run . --todas [limite]                # todas as empresas do CSV
go run . --scan                          # lista os .pfx e a senha extraída de cada nome
go run . --cert <raiz8>                  # mostra caminho e senha de uma raiz
go run . --resetar <raiz8|cnpj14>        # apaga o ponteiro e força rebaixar tudo
go run . <arquivo.pfx> <senha> [cnpj14]  # modo direto, sem CSV
```

O `--resetar` existe porque o programa decide o que baixar **pelo ponteiro, nunca olhando o disco**: apagar os XMLs sem apagar o ponteiro deixaria as notas inalcançáveis.

---

## Saída em disco

```
NFSE/
└── RAZAO SOCIAL_00000000/          ← raiz do CNPJ: agrupa matriz e filiais
    └── 00000000000100/             ← estabelecimento
        ├── 2026-07/                ← mês de emissão
        │   └── <chave-de-acesso>.xml
        └── _eventos/
            └── <chave-de-acesso>-<nsu>.xml
```

O mês vem da data de emissão lida do próprio XML, no formato `AAAA-MM` para ordenar cronologicamente no explorador de arquivos. Eventos (cancelamentos) levam o NSU no nome porque chegam com a **mesma chave de acesso** da nota que alteram.

---

## Limitações

- A API não filtra por data — a filtragem por competência é feita localmente, sobre os XMLs já baixados.
- Emitidas e recebidas chegam no mesmo fluxo, sem separação; distinguir exige comparar o CNPJ do prestador (embutido na chave de acesso) com o CNPJ consultado.
- Um download por vez, por decisão de projeto: o arquivo de ponteiro é único, e duas execuções simultâneas sobrescreveriam o registro uma da outra.
- A lista de filiais vem do CSV de clientes; não há descoberta automática de estabelecimentos.
- A reconversão de certificados BER depende do Windows.

---

## Stack

Go · TLS mútuo (mTLS) · PKCS#12 · REST · gzip/base64 · `net/http` · `html/template` · `//go:embed`

---

## Licença

MIT
