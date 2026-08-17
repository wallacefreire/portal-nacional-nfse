package main

import (
	"fmt"
	"sync"
)

type Tarefa struct {
	Raiz     string `json:"raiz"`
	Empresa  string `json:"empresa"`
	Situacao string `json:"situacao"`
	Salvos   int    `json:"salvos"`
	Falhas   int    `json:"falhas"`
	Mensagem string `json:"mensagem"`
	Pasta    string `json:"pasta"`
}

var (
	tarefasMutex sync.Mutex
	tarefas      = map[string]*Tarefa{}
)

func iniciarDownload(config Config, empresa *Company) (*Tarefa, error) {
	tarefasMutex.Lock()
	defer tarefasMutex.Unlock()

	for _, emAndamento := range tarefas {
		if emAndamento.Situacao == "rodando" {
			return nil, fmt.Errorf("Já existe um download rodando: %s", emAndamento.Empresa)
		}
	}

	tarefa := &Tarefa{
		Raiz:     empresa.Root,
		Empresa:  empresa.Name,
		Situacao: "rodando",
	}
	tarefas[empresa.Root] = tarefa

	go executarDownload(config, empresa, tarefa)

	return tarefa, nil
}

func executarDownload(config Config, empresa *Company, tarefa *Tarefa) {
	state, err := loadState(config.EstadoPath)
	if err != nil {
		concluir(tarefa, "erro", 0, 0, "", err.Error())
		return
	}

	resultado, err := downloadRoot(config, state, empresa.Root, empresa.CNPJs)
	if err != nil {
		concluir(tarefa, "erro", resultado.Salvos, resultado.Falhas, "", err.Error())
		return
	}

	if len(resultado.Pendencias) > 0 {
		pendencia := resultado.Pendencias[0]
		concluir(tarefa, "erro", resultado.Salvos, resultado.Falhas, "",
			pendencia.Tipo+": "+pendencia.Motivo)
		return

	}

	concluir(tarefa, "pronto", resultado.Salvos, resultado.Falhas, resultado.Pasta, "")
}

func concluir(tarefa *Tarefa, situacao string, salvos, falhas int, pasta, mensagem string) {
	tarefasMutex.Lock()
	defer tarefasMutex.Unlock()

	tarefa.Pasta = pasta
	tarefa.Situacao = situacao
	tarefa.Salvos = salvos
	tarefa.Falhas = falhas
	tarefa.Mensagem = mensagem
}

func lerTarefa(raiz string) *Tarefa {
	tarefasMutex.Lock()
	defer tarefasMutex.Unlock()

	tarefa := tarefas[raiz]
	if tarefa == nil {
		return nil
	}

	copia := *tarefa
	return &copia
}
