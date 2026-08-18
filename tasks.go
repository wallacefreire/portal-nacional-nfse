package main

import (
	"fmt"
	"sync"
)

type Task struct {
	Root    string `json:"raiz"`
	Company string `json:"empresa"`
	Status  string `json:"situacao"`
	Saved   int    `json:"salvos"`
	Failed  int    `json:"falhas"`
	Step    int    `json:"etapa"`
	Steps   int    `json:"etapas"`
	Message string `json:"mensagem"`
	Folder  string `json:"pasta"`
}

var (
	tasksMutex sync.Mutex
	tasks      = map[string]*Task{}
)

func startDownload(config Config, company *Company) (*Task, error) {
	tasksMutex.Lock()
	defer tasksMutex.Unlock()

	for _, running := range tasks {
		if running.Status == "rodando" {
			return nil, fmt.Errorf("Já existe um download rodando: %s", running.Company)
		}
	}

	task := &Task{
		Root:    company.Root,
		Company: company.Name,
		Status:  "rodando",
	}
	tasks[company.Root] = task

	go runDownload(config, company, task)

	return task, nil
}

func runDownload(config Config, company *Company, task *Task) {
	state, err := loadState(config.StatePath)
	if err != nil {
		finish(task, "erro", 0, 0, "", err.Error())
		return
	}

	result, err := downloadRoot(config, state, company.Root, company.CNPJs, task)
	if err != nil {
		finish(task, "erro", result.Saved, result.Failed, "", err.Error())
		return
	}

	if len(result.Issues) > 0 {
		issue := result.Issues[0]
		finish(task, "erro", result.Saved, result.Failed, "",
			issue.Kind+": "+issue.Reason)
		return

	}

	finish(task, "pronto", result.Saved, result.Failed, result.Folder, "")

	if result.Saved > 0 {
		openFolder(result.Folder)
	}
}

func finish(task *Task, status string, saved, failed int, folder, message string) {
	tasksMutex.Lock()
	defer tasksMutex.Unlock()

	task.Folder = folder
	task.Status = status
	task.Saved = saved
	task.Failed = failed
	task.Message = message
}

func (task *Task) progress(saved, failed int) {
	if task == nil {
		return
	}

	tasksMutex.Lock()
	defer tasksMutex.Unlock()

	task.Saved += saved
	task.Failed += failed
}

func (task *Task) startStep(step, steps int) {
	if task == nil {
		return
	}

	tasksMutex.Lock()
	defer tasksMutex.Unlock()

	task.Step = step
	task.Steps = steps
}

func readTask(root string) *Task {
	tasksMutex.Lock()
	defer tasksMutex.Unlock()

	task := tasks[root]
	if task == nil {
		return nil
	}

	clone := *task
	return &clone
}
