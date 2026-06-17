package web

import (
	"cmp"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/pipeline"
)

const defaultMaxUploadBytes = 100 << 20

//go:embed templates/*.html static/*.css
var assets embed.FS

type Runner interface {
	Run(context.Context, pipeline.RunInput) (domain.PipelineRun, error)
}

type Exporter interface {
	Export(context.Context, string) (string, error)
}

type Config struct {
	UploadDir       string
	OutputDir       string
	DefaultLanguage string
	MaxUploadBytes  int64
}

type Server struct {
	runner    Runner
	exporter  Exporter
	config    Config
	templates *template.Template
	logger    *log.Logger
}

type pageData struct {
	DefaultLanguage string
	Error           string
	Success         string
	DownloadURL     string
	DownloadName    string
}

func NewServer(runner Runner, exporter Exporter, config Config) (*Server, error) {
	if runner == nil {
		return nil, errors.New("runner is required")
	}
	if exporter == nil {
		return nil, errors.New("exporter is required")
	}
	config.UploadDir = cmp.Or(config.UploadDir, filepath.Join(os.TempDir(), "requirement-pipeline-uploads"))
	config.OutputDir = cmp.Or(config.OutputDir, "output")
	config.DefaultLanguage = cmp.Or(config.DefaultLanguage, "pt-BR")
	config.MaxUploadBytes = cmp.Or(config.MaxUploadBytes, int64(defaultMaxUploadBytes))

	templates, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{runner: runner, exporter: exporter, config: config, templates: templates, logger: log.Default()}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/process", s.process)
	mux.HandleFunc("/downloads/", s.download)
	static, _ := fs.Sub(assets, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	return mux
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.render(w, pageData{DefaultLanguage: s.config.DefaultLanguage})
}

func (s *Server) process(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioPath, originalName, err := s.saveUpload(w, r)
	if err != nil {
		s.render(w, pageData{DefaultLanguage: s.config.DefaultLanguage, Error: err.Error()})
		return
	}
	defer func() {
		if err := os.Remove(audioPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Printf("remove upload %s: %v", audioPath, err)
		}
	}()

	language := strings.TrimSpace(r.FormValue("language"))
	if language == "" {
		language = s.config.DefaultLanguage
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(originalName, filepath.Ext(originalName))
	}

	run, err := s.runner.Run(r.Context(), pipeline.RunInput{Title: title, AudioFile: audioPath, Language: language})
	if err != nil {
		s.logger.Printf("run pipeline: %v", err)
		s.render(w, pageData{DefaultLanguage: language, Error: "Nao foi possivel processar o audio. Verifique a configuracao e tente novamente."})
		return
	}

	pdfPath, err := s.exporter.Export(r.Context(), run.ID)
	if err != nil {
		s.logger.Printf("export pdf: %v", err)
		s.render(w, pageData{DefaultLanguage: language, Error: "O processamento terminou, mas nao foi possivel gerar o PDF."})
		return
	}

	name := filepath.Base(pdfPath)
	s.render(w, pageData{
		DefaultLanguage: language,
		Success:         "PDF gerado com sucesso.",
		DownloadURL:     "/downloads/" + url.PathEscape(name),
		DownloadName:    name,
	})
}

func (s *Server) saveUpload(w http.ResponseWriter, r *http.Request) (string, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", "", fmt.Errorf("selecione um arquivo de audio")
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		return "", "", fmt.Errorf("selecione um arquivo de audio")
	}
	defer file.Close()
	if header.Filename == "" || header.Size == 0 {
		return "", "", fmt.Errorf("selecione um arquivo de audio")
	}

	if err := os.MkdirAll(s.config.UploadDir, 0o755); err != nil {
		return "", "", fmt.Errorf("nao foi possivel preparar o diretorio de upload")
	}
	ext := filepath.Ext(header.Filename)
	path := filepath.Join(s.config.UploadDir, domain.NewID()+ext)
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("nao foi possivel salvar o upload")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("nao foi possivel salvar o upload")
	}
	return path, filepath.Base(header.Filename), nil
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, err := safePDFName(strings.TrimPrefix(r.URL.Path, "/downloads/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	root, err := os.OpenRoot(s.config.OutputDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeFileFS(w, r, root.FS(), name)
}

func safePDFName(value string) (string, error) {
	name, err := url.PathUnescape(value)
	if err != nil {
		return "", err
	}
	if name == "" || name != filepath.Base(name) || filepath.Ext(name) != ".pdf" || strings.Contains(name, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid pdf name")
	}
	return name, nil
}

func (s *Server) render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		s.logger.Printf("render template: %v", err)
	}
}
