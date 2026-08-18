package main

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/exec"
	"sort"
)

//go:embed logo-branco.png
var logoPNG []byte

//go:embed fundo.png
var fundoPNG []byte

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>NFS-e | Conta Comigo</title>
<style>
  :root {
    --azul: #0B2A8C;
    --azul-fundo: #071D63;
    --verde: #5CB944;
    --lima: #A3D53C;
    --cinza: #f4f6fb;
    --borda: #e1e5f0;
    --texto: #1c2033;
  }

    * { box-sizing: border-box; }

  body {
    margin: 0;
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    color: var(--texto);
    background: var(--cinza);
  }

  header {
    background: linear-gradient(135deg, var(--azul-fundo), var(--azul));
    border-bottom: 4px solid var(--verde);
    padding: .85rem 2rem .85rem 8.5rem;
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .marca {
    height: 100px;
    width: auto;
    display: block;
  }

  .app {
    color: #fff;
    font-size: 1.05rem;
    font-weight: 500;
    padding-left: 1.5rem;
    border-left: 1px solid rgba(255, 255, 255, .28);
  }

  main {
    max-width: 760px;
    margin: 2rem auto;
    padding: 0 1.5rem;
  }

  h1 {
    font-size: 1.3rem;
    margin: 0 0 1rem;
  }

  .busca-area {
    position: sticky;
    top: 0;
    background: var(--cinza);
    padding: 1rem 0 .6rem;
    z-index: 10;
  }

  .campo { position: relative; }

  #busca {
    width: 100%;
    padding: .85rem 2.9rem .85rem 1rem;
    font-size: 1rem;
    border: 1px solid var(--borda);
    border-radius: 10px;
    background: #fff;
    transition: border-color .15s, box-shadow .15s;
  }

  #limpar {
    position: absolute;
    right: .55rem;
    top: 50%;
    transform: translateY(-50%);
    width: 1.8rem;
    height: 1.8rem;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: #8b90a6;
    font-size: 1.35rem;
    line-height: 1;
    cursor: pointer;
    transition: background .12s, color .12s;
  }

  #limpar:hover {
    background: #eef0f7;
    color: var(--texto);
  }

  #limpar[hidden] { display: none; }

  #busca:focus {
    outline: none;
    border-color: var(--azul);
    box-shadow: 0 0 0 3px rgba(11, 42, 140, .12);
  }

  #contador {
    font-size: .85rem;
    color: #6b7189;
    margin: .7rem 0 0;
    text-align: right;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    background: #fff;
    border-radius: 12px;
    box-shadow: 0 1px 2px rgba(16, 24, 64, .06),
                0 4px 16px rgba(16, 24, 64, .06);
  }

  li {
    position: relative;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: .9rem 1.1rem;
    background: #fff;
    border-bottom: 1px solid #f0f2f8;
    cursor: pointer;
    transition: transform .14s ease, box-shadow .14s ease;
  }

  li:first-child { border-radius: 12px 12px 0 0; }

  li:last-child {
    border-bottom: none;
    border-radius: 0 0 12px 12px;
  }

  li:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(16, 24, 64, .13);
    border-radius: 10px;
    z-index: 2;
  }
  
  li[hidden] { display: none; }

  .nome { font-weight: 500; }

  .raiz {
    color: #7a8099;
    font-size: .85rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .filiais {
    color: var(--verde);
    font-weight: 600;
    margin-left: .5rem;
  }

    .direita {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .acao {
    min-width: 7rem;
    text-align: center;
    padding: .4rem .9rem;
    border-radius: 999px;
    font-size: .82rem;
    font-weight: 600;
    background: #fff;
    color: var(--verde);
    border: 1px solid var(--verde);
  }

    li:hover .acao {
    background: var(--verde);
    color: #fff;
  }

  .acao.rodando,
  li:hover .acao.rodando {
    background: #eef0f7;
    color: #6b7189;
    border-color: #dfe3ee;
  }

  .acao.pronto,
  li:hover .acao.pronto {
    background: #e8f6e6;
    color: #2f7d20;
    border-color: #bfe3ba;
  }

  .acao.erro,
  li:hover .acao.erro {
    background: #fdeaea;
    color: #a52020;
    border-color: #f2c4c4;
  }
</style>
</head>
<body>

<header>
  <img src="/logo.png" alt="Conta Comigo" class="marca">
  <span class="app">Notas Fiscais de Serviço</span>
</header>

<main>
  <h1>Escolha a empresa</h1>
  <div class="busca-area">
    <div class="campo">
      <input id="busca" placeholder="Nome ou CNPJ da empresa" autofocus>
      <button id="limpar" type="button" aria-label="Limpar busca" hidden>&times;</button>
    </div>
    <p id="contador"></p>
  </div>
  <ul id="lista">
  {{range .}}
    <li data-busca="{{.Name}} {{.Root}}" data-raiz="{{.Root}}">
      <span class="nome">{{.Name}}</span>
        <span class="direita">
        <span class="raiz">{{.Root}}{{if gt (len .CNPJs) 1}}<span class="filiais">{{len .CNPJs}} estab.</span>{{end}}</span>
        <span class="acao">Baixar</span>
      </span>
    </li>

  {{end}}
  </ul>
</main>

<script>
const busca = document.getElementById('busca');
const limpar = document.getElementById('limpar');
const contador = document.getElementById('contador');
const itens = Array.from(document.querySelectorAll('#lista li'));

function filtrar() {
  const termo = busca.value.toLowerCase();
  limpar.hidden = termo === '';

  let visiveis = 0;
  for (const li of itens) {
    const mostra = li.dataset.busca.toLowerCase().includes(termo);
    li.hidden = !mostra;
    if (mostra) visiveis++;
  }
  contador.textContent = visiveis + (visiveis === 1 ? ' empresa' : ' empresas');
}

function limparBusca() {
  busca.value = '';
  busca.focus();
  filtrar();
}

busca.addEventListener('input', filtrar);
limpar.addEventListener('click', limparBusca);
busca.addEventListener('keydown', (evento) => {
  if (evento.key === 'Escape') limparBusca();
});
filtrar();

function mostrar(acao, texto, situacao) {
  acao.textContent = texto;
  acao.className = situacao ? 'acao ' + situacao : 'acao';
}

function acompanhar(li) {
  const acao = li.querySelector('.acao');

  const relogio = setInterval(async () => {
    const resposta = await fetch('/status?raiz=' + li.dataset.raiz);
    if (!resposta.ok) return;

    const tarefa = await resposta.json();
    if (tarefa.situacao === 'rodando') return;

    clearInterval(relogio);
    li.dataset.ocupado = 'nao';

    if (tarefa.situacao === 'erro') {
      mostrar(acao, 'Falhou', 'erro');
      li.title = tarefa.mensagem;
      return;
    }

    mostrar(acao, tarefa.salvos > 0 ? tarefa.salvos + ' notas' : 'Nada novo', 'pronto');

  }, 2000);
}

for (const li of itens) {
  li.addEventListener('click', async () => {
    if (li.dataset.ocupado === 'sim') return;

    const acao = li.querySelector('.acao');
    li.dataset.ocupado = 'sim';
    mostrar(acao, 'Iniciando...', 'rodando');

    const resposta = await fetch('/baixar?raiz=' + li.dataset.raiz);
    if (!resposta.ok) {
      mostrar(acao, (await resposta.text()).trim(), 'erro');
      li.dataset.ocupado = 'nao';
      return;
    }

    mostrar(acao, 'Baixando...', 'rodando');
    acompanhar(li);
  });
}

</script>
</body>
</html>`))

func serveWeb(config Config, address string) error {
	companies, err := loadCompanies(config.ClientsCSV)
	if err != nil {
		return err
	}

	list := make([]*Company, 0, len(companies))
	for _, company := range companies {
		list = append(list, company)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := page.Execute(w, list); err != nil {
			log.Println("erro ao montar a página:", err)
		}
	})

	http.HandleFunc("/baixar", func(w http.ResponseWriter, r *http.Request) {
		company := companies[r.URL.Query().Get("raiz")]
		if company == nil {
			http.Error(w, "Empresa não encontrada", http.StatusNotFound)
			return
		}

		task, err := startDownload(config, company)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		respondJSON(w, task)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		task := readTask(r.URL.Query().Get("raiz"))
		if task == nil {
			http.Error(w, "Nenhum download para essa empresa", http.StatusNotFound)
			return
		}

		respondJSON(w, task)
	})

	http.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(logoPNG)
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	log.Println("servidor no ar em http://" + address + " - Ctrl+C para encerrar")
	openBrowser("http://" + address)

	return http.Serve(listener, nil)
}

func respondJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("erro ao responder JSON:", err)
	}
}

func openBrowser(url string) {
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		log.Println("Não consegui abrir o navegador. Acesse " + url + " manualmente.")
	}
}
