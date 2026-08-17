# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Código, comentários, mensagens de erro e testes estão em **português**. Mantenha esse padrão.

## Comandos

```bash
go build .                              # compilar
go test ./...                           # todos os testes
go test -run TestExtractPassword        # um teste só

go run . --scan                         # lista os .pfx e a senha extraída de cada nome
go run . --cert <raiz8>                 # mostra caminho e senha de uma raiz
go run . --empresa <raiz8> [cnpj14...]  # baixa uma empresa; sem cnpj, lê os do CSV
go run . --todas [limite]               # todas as empresas do CSV; limite corta a lista
go run . --resetar <raiz8|cnpj14>       # apaga o ponteiro; a próxima execução rebaixa tudo
go run . --tela                         # interface web em http://localhost:8080
go run . <arquivo.pfx> <senha> [cnpj14] # modo direto, sem CSV
```

O `--tela` embute o HTML no binário, então **mudança na página exige reiniciar o servidor** (Ctrl+C e rodar de novo) — o processo antigo continua servindo a versão velha.

`--empresa <raiz8> <cnpj14>` é o modo de teste mais útil: pula o CSV e baixa um CNPJ só.

O programa decide o que baixar **só pelo ponteiro, nunca olhando o disco** — apagar XMLs sem apagar o ponteiro deixa as notas inalcançáveis, e o `--resetar` é o que desfaz isso. Ele aceita raiz de 8 dígitos (pega todos os estabelecimentos) ou o CNPJ completo, porque a raiz é o começo do CNPJ e um `strings.HasPrefix` atende os dois casos.

## Arquitetura

Package `main` único, arquivos divididos por assunto: `adn.go` (HTTP e API), `certificado.go` (.pfx e mTLS), `clientes.go` (CSV), `config.go`, `download.go` (orquestração e gravação), `estado.go` (ponteiro), `main.go` (modos de linha de comando), `web.go` (a página e as rotas), `tarefas.go` (downloads em segundo plano).

### A interface web

Alvo final do projeto: os auxiliares contábeis operam por tela, não por linha de comando. `servirWeb` lê o CSV **uma vez na subida**, ordena e guarda numa closure — a busca das ~237 empresas acontece no navegador, não no servidor.

O download não pode responder na mesma requisição: a primeira carga de uma empresa leva minutos e o navegador desiste antes. Então `/baixar` dispara uma **goroutine** e responde na hora; a página pergunta `/status` a cada 2s. O estado fica num `map[raiz]*Tarefa` protegido por mutex, e `lerTarefa` devolve **cópia**, nunca o ponteiro.

**Um download por vez, de propósito** (`iniciarDownload` recusa se houver outro rodando): o `nsu.json` é um arquivo só, e dois downloads simultâneos sobrescreveriam o ponteiro um do outro. Impedir é mais barato que administrar.

⚠️ O HTML/CSS/JS vive dentro de uma **string crua do Go delimitada por crase** — não use crase no JavaScript (nada de template literal), ela encerra a string. Monte URL com `+`.

⚠️ `hidden` não funciona em elemento com `display` declarado. Toda vez que o JS usar `hidden`, precisa da regra `[hidden] { display: none; }` junto — já pegou o `li` da lista e o botão de limpar a busca.

### O modelo NSU — o conceito que explica o resto

O ADN não aceita consulta por data. É uma **caixa postal numerada**: cada documento destinado a um CNPJ ganha um NSU sequencial, e a única operação é *"me dê o que chegou depois do NSU X"* (`GET /contribuintes/DFe/{NSU}?cnpjConsulta=`).

Três consequências que orientam o código todo:

- **O ponteiro é por CNPJ, não por certificado.** Cada estabelecimento tem caixa e numeração próprias — uma matriz com 3 filiais são 4 ponteiros independentes.
- **O certificado da matriz baixa as filiais** via `cnpjConsulta`. O e-CNPJ representa a pessoa jurídica inteira. É a razão de o projeto existir.
- **Emitidas e recebidas vêm no mesmo fluxo**, misturadas com eventos. Não há filtro. Para separar, compare o CNPJ do prestador (dígitos 10 a 23 da chave de acesso) com o CNPJ consultado.

### Certificados

- A **senha está no nome do arquivo** `.pfx`, em padrões inconsistentes (`--`, `-`, "senha"/"Senha"/"SENHA"). `extractPasswordFromFilename` cobre os casos reais; `main_test.go` tem exemplos verdadeiros — acrescente lá antes de mexer na função.
- Certificados ICP-Brasil vêm em **BER** e o Go só lê **DER**. `loadCertificate` tenta ler direto, cai para um cache em `convertidosDir` e, em último caso, reconverte chamando **`pwsh`** (PowerShell 7 — `powershell.exe` 5.1 não serve). Conversão em Go puro foi tentada e falhou; não repita.
- Nunca coloque a senha em mensagem de erro.

### Estado

`NSUState` é um `map[cnpj]nsu` em JSON. Gravado **a cada lote** com escrita atômica (temp + rename). Se a gravação falhar, `ErrEstadoNaoSalvo` **aborta a execução inteira de propósito** — continuar sem conseguir registrar onde parou causa perda ou reprocessamento.

### Saída em disco

```
<xmlBaseDir>/<NOME>_<raiz8>/<cnpj14>/<AAAA-MM>/<chave>.xml
<xmlBaseDir>/<NOME>_<raiz8>/<cnpj14>/_eventos/<chave>-<nsu>.xml
```

O mês vem de `<dhProc>` (emissão, não competência), formato `AAAA-MM` para ordenar no Explorer. Sem `dhProc`, cai em `sem-data`. Eventos levam o NSU no nome porque **chegam com a mesma `ChaveAcesso` da nota**.

⚠️ A regra que monta `<NOME>_<raiz8>` existe em **dois lugares**: `downloadCNPJ`, que grava de fato, e `downloadRoot`, que só calcula para informar em `Resultado.Pasta` — é o caminho que a tela mostra ao auxiliar. Mudou uma, mude a outra.

### Configuração

`config.json` (fora do versionamento). Campos vazios acionam os padrões em `aplicarPadroes`, que também **cria as pastas**: `~/Documents/NFSE` e `~/Documents/NFSE/_controle/nsu.json`. É o que permite rodar numa máquina nova sem configurar nada.

## Restrições que já custaram caro

1. **`estadoPath` nunca no Google Drive.** O drive virtual não substitui arquivo existente; o `os.Rename` falha e derruba o programa (acontecia por volta do lote 11). Estado é arquivo de trabalho — disco local.
2. **Gravar XML direto no Drive não é confiável.** Já produziu `0 falhas` com nada chegando ao destino: o Windows aceita a gravação e o cliente do Google falha depois, em silêncio, desviando arquivos para `Meu Drive` e criando pastas duplicadas de mesmo nome. Provado em 13/08/2026: mesmo código, destino local, 12.476/12.476 e uma pasta só.
3. **A pausa entre lotes é 1s — medida, não chutada.** Constante `pausaEntreLotes` no `download.go`. Com 500ms o ADN devolve 429 na segunda requisição; com 1s rodam 13 lotes seguidos sem nenhum. A busca em si leva 0,2–0,4s, então o gargalo é a pausa e não a rede. Descer abaixo de 1s economiza poucos segundos e um único 429 custa 5s de backoff (que dobra, até 6 tentativas) — não compensa. O valor anterior era 2s, escolhido com folga logo depois de o 500ms falhar, e nunca tinha sido medido.
4. **Lote = 50, fixo pelo servidor.** O parâmetro `lote` do swagger é **booleano** (lote sim/não), não tamanho. Não existe página de 100.
5. **`MkdirAll` uma vez por pasta, não por arquivo.** Já foram 12.456 chamadas numa execução. `gravarComTentativas` recebe um `map[string]bool`; o `delete` após falha força recriar (o Drive some com pastas).
6. **`savedCount` conta gravações, não arquivos.** Documentos com a mesma `ChaveAcesso` sobrescrevem — 53 salvos podem virar 51 arquivos. Qualquer conferência que compare os dois números direto dá alarme falso.
7. **Na dúvida entre repetir e pular um NSU, repita.** Arquivo é sobrescrito; nota pulada some para sempre. O ponteiro só avança quando o lote inteiro gravou (`falhasDeGravacao == 0`).
8. **Pendência não é erro.** `downloadRoot` devolve `err == nil` **com uma `Pendencia` dentro** quando o certificado está vencido, não abre ou não existe. É de propósito: no `--todas`, uma empresa quebrada não pode parar as outras 236. Quem consome o resultado precisa checar `len(resultado.Pendencias) > 0`, não só o `err`. Isso já causou bug — a tela mostrava pílula **verde** "Nada novo" para certificado vencido.

## README desatualizado

O `README.md` descreve a estrutura de saída antiga (`NFSE/` e `EVENTO/` por empresa), não menciona `--empresa`, `--todas` nem `clientesCSV`, e omite os padrões automáticos de configuração. Prefira este arquivo; atualize o README quando mexer nessas áreas.
