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

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/pipeline"
	"github.com/fabiuhp/tcc-3/internal/transcripts"
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
	Warning         string
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

	input, originalName, err := s.saveUpload(w, r)
	if err != nil {
		s.render(w, pageData{DefaultLanguage: s.config.DefaultLanguage, Error: err.Error()})
		return
	}
	defer func() {
		if input.AudioFile == "" {
			return
		}
		if err := os.Remove(input.AudioFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Printf("remove upload %s: %v", input.AudioFile, err)
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

	input.Title = title
	input.Language = language
	run, pipelineErr := s.runner.Run(r.Context(), input)
	if pipelineErr != nil {
		s.logger.Printf("pipeline failed run_id=%s error=%v", run.ID, pipelineErr)
		if run.ID == "" {
			s.render(w, pageData{DefaultLanguage: language, Error: formatPipelineError(pipelineErr)})
			return
		}
	}
	for _, issue := range run.ValidationIssues {
		s.logger.Printf("pipeline validation issue run_id=%s stage=%s item=%s field=%s code=%s action=%s value=%q message=%s", run.ID, issue.Stage, issue.Item, issue.Field, issue.Code, issue.Action, issue.Value, issue.Message)
	}

	markdownPath, exportErr := s.exporter.Export(r.Context(), run.ID)
	if exportErr != nil {
		s.logger.Printf("export markdown run_id=%s error=%v", run.ID, exportErr)
		message := fmt.Sprintf("Nao foi possivel gerar o Markdown: %v", exportErr)
		if pipelineErr != nil {
			message = fmt.Sprintf("%s. %s", formatPipelineError(pipelineErr), message)
		}
		s.render(w, pageData{DefaultLanguage: language, Error: message})
		return
	}

	name := filepath.Base(markdownPath)
	success := "Markdown gerado com sucesso."
	if pipelineErr != nil {
		success = "Markdown parcial gerado com ressalvas. Consulte o diagnostico no documento."
	} else if run.Status == domain.StatusCompletedWithIssues {
		success = "Markdown gerado com ressalvas. Consulte os problemas de validacao no documento."
	}
	data := pageData{
		DefaultLanguage: language,
		Success:         success,
		DownloadURL:     "/downloads/" + url.PathEscape(name),
		DownloadName:    name,
	}
	if pipelineErr != nil {
		data.Error = formatPipelineError(pipelineErr)
	}
	if len(run.ValidationIssues) > 0 {
		data.Warning = formatValidationIssues(run.ValidationIssues)
	}
	s.render(w, pageData{
		DefaultLanguage: data.DefaultLanguage,
		Error:           data.Error,
		Warning:         data.Warning,
		Success:         data.Success,
		DownloadURL:     data.DownloadURL,
		DownloadName:    data.DownloadName,
	})
}

func formatPipelineError(err error) string {
	var stageErr pipeline.StageError
	if errors.As(err, &stageErr) {
		return fmt.Sprintf("Falha na etapa %q: %s", stageErr.Stage, stageErr.Err)
	}
	return fmt.Sprintf("Falha no processamento: %s", err)
}

func formatValidationIssues(issues []domain.ValidationIssue) string {
	details := make([]string, 0, len(issues))
	for _, issue := range issues {
		locationParts := []string{}
		if issue.Stage != "" {
			locationParts = append(locationParts, string(issue.Stage))
		}
		if issue.Item != "" {
			locationParts = append(locationParts, issue.Item)
		}
		if issue.Field != "" {
			locationParts = append(locationParts, issue.Field)
		}
		location := strings.Join(locationParts, " / ")
		if location == "" {
			location = issue.Code
		}
		message := issue.Message
		if issue.Value != "" {
			message = fmt.Sprintf("%s Valor rejeitado: %q", message, issue.Value)
		}
		if issue.Action != "" {
			message = fmt.Sprintf("%s Acao tomada: %s", message, issue.Action)
		}
		details = append(details, fmt.Sprintf("%s: %s", location, message))
	}
	return "Processamento concluido com ressalvas: " + strings.Join(details, " | ")
}

func (s *Server) saveUpload(w http.ResponseWriter, r *http.Request) (pipeline.RunInput, string, error) {
	var input pipeline.RunInput
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes)
	err := r.ParseMultipartForm(32 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			return input, "", fmt.Errorf("o upload excede o limite de %d bytes", s.config.MaxUploadBytes)
		}
		return input, "", fmt.Errorf("selecione um arquivo de audio ou uma transcricao .md")
	}
	field := "audio"
	audioFiles := r.MultipartForm.File["audio"]
	transcriptFiles := r.MultipartForm.File["transcript"]
	if len(audioFiles)+len(transcriptFiles) != 1 {
		return input, "", fmt.Errorf("envie apenas um arquivo: audio ou transcricao .md")
	}
	if len(transcriptFiles) == 1 {
		field = "transcript"
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		return input, "", fmt.Errorf("selecione um arquivo de audio ou uma transcricao .md")
	}
	defer file.Close()
	if header.Filename == "" || header.Size == 0 {
		return input, "", fmt.Errorf("o arquivo esta vazio; selecione um audio ou uma transcricao .md com conteudo")
	}
	name := filepath.Base(header.Filename)
	if field == "transcript" {
		text, err := transcripts.ReadMarkdown(name, file)
		if err != nil {
			return input, "", err
		}
		return pipeline.RunInput{Transcript: text, TranscriptFile: name}, name, nil
	}

	if err := os.MkdirAll(s.config.UploadDir, 0o755); err != nil {
		return input, "", fmt.Errorf("nao foi possivel preparar o diretorio de upload")
	}
	ext := filepath.Ext(header.Filename)
	path := filepath.Join(s.config.UploadDir, domain.NewID()+ext)
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return input, "", fmt.Errorf("nao foi possivel salvar o upload")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(path)
		return input, "", fmt.Errorf("nao foi possivel salvar o upload")
	}
	return pipeline.RunInput{AudioFile: path}, name, nil
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, err := safeMarkdownName(strings.TrimPrefix(r.URL.Path, "/downloads/"))
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

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeFileFS(w, r, root.FS(), name)
}

func safeMarkdownName(value string) (string, error) {
	name, err := url.PathUnescape(value)
	if err != nil {
		return "", err
	}
	if name == "" || name != filepath.Base(name) || filepath.Ext(name) != ".md" || strings.Contains(name, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid markdown name")
	}
	return name, nil
}

func (s *Server) render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		s.logger.Printf("render template: %v", err)
	}
}
