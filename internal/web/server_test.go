package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/pipeline"
)

func TestServerRendersUploadForm(t *testing.T) {
	server := newTestServer(t, &fakeRunner{}, &fakeExporter{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, want := range []string{"Gerar documento de requisitos", "name=\"audio\"", "Processar audio"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
}

func TestServerRendersProcessingFeedback(t *testing.T) {
	server := newTestServer(t, &fakeRunner{}, &fakeExporter{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.Handler().ServeHTTP(recorder, request)

	body := recorder.Body.String()
	for _, want := range []string{
		`id="processing-state"`,
		`role="status"`,
		`aria-live="polite"`,
		`form.setAttribute('aria-busy', 'true')`,
		`HTMLFormElement.prototype.submit.call(form)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing processing feedback %q", want)
		}
	}
}

func TestServerServesProcessingStyles(t *testing.T) {
	server := newTestServer(t, &fakeRunner{}, &fakeExporter{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	for _, want := range []string{".processing-state", ".processing-spinner", "@keyframes processing-spin"} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("stylesheet missing %q", want)
		}
	}
}

func TestServerRejectsMissingAudio(t *testing.T) {
	runner := &fakeRunner{}
	server := newTestServer(t, runner, &fakeExporter{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/process", strings.NewReader("title=Discovery"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
	if !strings.Contains(recorder.Body.String(), "selecione um arquivo de audio") {
		t.Fatalf("response did not include validation message")
	}
}

func TestServerProcessesUploadAndReturnsDownload(t *testing.T) {
	runner := &fakeRunner{runID: "run-123"}
	exporter := &fakeExporter{}
	server := newTestServer(t, runner, exporter)
	body, contentType := multipartBody(t, "audio", "meeting.mp3", "fake audio")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/process", body)
	request.Header.Set("Content-Type", contentType)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if runner.lastInput.AudioFile == "" {
		t.Fatal("runner did not receive uploaded audio path")
	}
	if _, err := os.Stat(runner.lastInput.AudioFile); !os.IsNotExist(err) {
		t.Fatalf("upload file was not cleaned up, stat err = %v", err)
	}
	if exporter.runID != "run-123" {
		t.Fatalf("exporter run id = %q, want run-123", exporter.runID)
	}
	if !strings.Contains(recorder.Body.String(), "/downloads/result.md") {
		t.Fatalf("response did not include download link")
	}
}

func TestServerExportsMarkdownAfterPipelineFailure(t *testing.T) {
	runner := &fakeRunner{runID: "run-failed", err: pipeline.StageError{Stage: domain.StageRequirementExtraction, Err: errors.New("evidence validation failed")}}
	server := newTestServer(t, runner, &fakeExporter{})
	body, contentType := multipartBody(t, "audio", "meeting.mp3", "fake audio")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/process", body)
	request.Header.Set("Content-Type", contentType)

	server.Handler().ServeHTTP(recorder, request)

	response := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	for _, want := range []string{"Falha na etapa", "requirement_extraction", "evidence validation failed", "Markdown parcial gerado", "/downloads/result.md"} {
		if !strings.Contains(response, want) {
			t.Fatalf("response missing failure fallback detail %q", want)
		}
	}
}

func TestServerShowsValidationIssuesAndReturnsDownload(t *testing.T) {
	runner := &fakeRunner{
		runID: "run-with-issues",
		validationIssues: []domain.ValidationIssue{{
			Stage:   domain.StageRequirementExtraction,
			Code:    "evidence_quote_unverifiable",
			Item:    "REQ-0001",
			Field:   "evidence[1].quote",
			Message: "The source quote does not occur in the transcript and was removed.",
		}},
	}
	server := newTestServer(t, runner, &fakeExporter{})
	body, contentType := multipartBody(t, "audio", "meeting.mp3", "fake audio")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/process", body)
	request.Header.Set("Content-Type", contentType)

	server.Handler().ServeHTTP(recorder, request)

	response := recorder.Body.String()
	for _, want := range []string{"Processamento concluido com ressalvas", "REQ-0001", "evidence[1].quote", "/downloads/result.md"} {
		if !strings.Contains(response, want) {
			t.Fatalf("response missing validation detail %q", want)
		}
	}
}

func TestServerServesMarkdownDownloadWithHeaders(t *testing.T) {
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "result.md"), []byte("# Requirements"), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	server, err := NewServer(&fakeRunner{}, &fakeExporter{}, Config{UploadDir: t.TempDir(), OutputDir: outputDir, DefaultLanguage: "pt-BR"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/downloads/result.md", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "result.md") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/markdown") {
		t.Fatalf("Content-Type = %q, want text/markdown", got)
	}
}

func TestServerRejectsUnsafeDownloadPath(t *testing.T) {
	server := newTestServer(t, &fakeRunner{}, &fakeExporter{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/downloads/%2e%2e%2fsecret.md", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func newTestServer(t *testing.T, runner Runner, exporter Exporter) *Server {
	t.Helper()
	server, err := NewServer(runner, exporter, Config{UploadDir: t.TempDir(), OutputDir: t.TempDir(), DefaultLanguage: "pt-BR"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func multipartBody(t *testing.T, field string, name string, content string) (io.Reader, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

type fakeRunner struct {
	calls            int
	runID            string
	lastInput        pipeline.RunInput
	err              error
	validationIssues []domain.ValidationIssue
}

func (f *fakeRunner) Run(_ context.Context, input pipeline.RunInput) (domain.PipelineRun, error) {
	f.calls++
	f.lastInput = input
	if f.err != nil {
		return domain.PipelineRun{ID: f.runID, Status: domain.StatusFailed}, f.err
	}
	if f.runID == "" {
		f.runID = "run-id"
	}
	return domain.PipelineRun{ID: f.runID, Status: domain.StatusCompleted, ValidationIssues: f.validationIssues}, nil
}

type fakeExporter struct {
	runID string
	err   error
}

func (f *fakeExporter) Export(_ context.Context, runID string) (string, error) {
	f.runID = runID
	if f.err != nil {
		return "", f.err
	}
	return filepath.Join(os.TempDir(), "result.md"), nil
}
