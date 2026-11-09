package main

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

//go:embed assets/logo-branco.png
var logoPNG []byte

//go:embed page.html
var pageHTML string

var page = template.Must(template.New("page").Parse(pageHTML))

func serveWeb(config Config, address string, showPage bool) error {
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
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		company := companies[r.URL.Query().Get("raiz")]
		if company == nil {
			http.Error(w, "Empresa não encontrada", http.StatusNotFound)
			return
		}

		task, err := startDownload(config, company, r.URL.Query().Get("competencia"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		respondJSON(w, task)
	})

	http.HandleFunc("/mes", func(w http.ResponseWriter, r *http.Request) {
		company, companyDir, competence, ok := monthRequest(w, r, companies)
		if !ok {
			return
		}

		respondJSON(w, summarizeMonth(companyDir, company.CNPJs, competence))
	})

	http.HandleFunc("/abrir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		company, companyDir, competence, ok := monthRequest(w, r, companies)
		if !ok {
			return
		}

		cnpj := r.URL.Query().Get("cnpj")
		if !slices.Contains(company.CNPJs, cnpj) {
			http.Error(w, "CNPJ não pertence à empresa", http.StatusBadRequest)
			return
		}

		monthDir := filepath.Join(companyDir, cnpj, competence)
		if _, err := os.Stat(monthDir); err != nil {
			http.Error(w, "Nenhuma nota nesse mês", http.StatusNotFound)
			return
		}

		openFolder(monthDir)
		w.WriteHeader(http.StatusNoContent)
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

	url := "http://" + address

	listener, err := net.Listen("tcp", address)
	if err != nil {
		running, dialErr := net.DialTimeout("tcp", address, time.Second)
		if dialErr != nil {
			return err
		}
		running.Close()

		if showPage {
			log.Println("Já existe uma instância no ar - abrindo " + url)
			openBrowser(url)
		}
		return nil
	}

	log.Println("servidor no ar em " + url + " - Ctrl+C para encerrar")
	if showPage {
		openBrowser(url)
	}

	return http.Serve(listener, nil)
}

func monthRequest(w http.ResponseWriter, r *http.Request, companies map[string]*Company) (*Company, string, string, bool) {
	root := r.URL.Query().Get("raiz")
	company := companies[root]
	if company == nil {
		http.Error(w, "Empresa não encontrada", http.StatusNotFound)
		return nil, "", "", false
	}

	competence := r.URL.Query().Get("competencia")
	if !validCompetence(competence) {
		http.Error(w, "Competência inválida", http.StatusBadRequest)
		return nil, "", "", false
	}

	task := readTask(root)
	if task == nil || task.Folder == "" {
		http.Error(w, "Baixe a empresa primeiro", http.StatusNotFound)
		return nil, "", "", false
	}

	return company, task.Folder, competence, true
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

func openFolder(path string) {
	if err := exec.Command("explorer", path).Start(); err != nil {
		log.Println("Impedido de abrir a pasta:", path)
	}
}
