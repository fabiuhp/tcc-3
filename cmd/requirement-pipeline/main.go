package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"requirement-pipeline/internal/config"
	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/export"
	"requirement-pipeline/internal/pipeline"
	"requirement-pipeline/internal/ports"
	"requirement-pipeline/internal/prompts"
	"requirement-pipeline/internal/providers"
	"requirement-pipeline/internal/repository"
	"requirement-pipeline/internal/stages"
	"requirement-pipeline/internal/web"
)

func main() {
	title := flag.String("title", "Untitled meeting", "meeting title")
	audio := flag.String("audio", "", "audio file path")
	language := flag.String("language", "", "meeting language")
	meetingID := flag.String("meeting-id", "", "existing meeting id to reprocess")
	outputDir := flag.String("out", "output", "directory for generated files")
	exportRunID := flag.String("export-run-id", "", "existing pipeline run id to export requirements Markdown without reprocessing")
	serveWeb := flag.Bool("web", false, "start the browser upload frontend instead of running the CLI pipeline")
	addr := flag.String("addr", ":8080", "HTTP address for -web mode")
	uploadDir := flag.String("uploads", "uploads", "directory for temporary web uploads")
	flag.Parse()

	cfg := config.Load()
	if *language == "" {
		*language = cfg.DefaultLanguage
	}

	ctx := context.Background()
	client, store, err := repository.ConnectMongo(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer client.Disconnect(ctx)

	if *serveWeb {
		runner, err := buildRunner(ctx, store, cfg)
		if err != nil {
			log.Fatalf("initialize web pipeline: %v", err)
		}
		server, err := web.NewServer(runner, web.FileExporter{Store: store, OutputDir: *outputDir}, web.Config{UploadDir: *uploadDir, OutputDir: *outputDir, DefaultLanguage: cfg.DefaultLanguage})
		if err != nil {
			log.Fatalf("initialize web server: %v", err)
		}
		log.Printf("serving frontend at http://localhost%s", *addr)
		if err := http.ListenAndServe(*addr, server.Handler()); err != nil {
			log.Fatalf("serve frontend: %v", err)
		}
		return
	}

	if *exportRunID != "" {
		markdownPath, err := exportRun(ctx, store, *exportRunID, *outputDir)
		if err != nil {
			log.Fatalf("export requirements Markdown: %v", err)
		}
		fmt.Printf("requirements Markdown generated: %s\n", markdownPath)
		return
	}

	runner, err := buildRunner(ctx, store, cfg)
	if err != nil {
		log.Fatalf("initialize pipeline: %v", err)
	}

	run, err := runner.Run(ctx, pipeline.RunInput{MeetingID: *meetingID, Title: *title, AudioFile: *audio, Language: *language})
	if err != nil {
		log.Fatalf("run pipeline: %v", err)
	}
	fmt.Printf("pipeline run completed: %s\n", run.ID)

	markdownPath, err := exportRun(ctx, store, run.ID, *outputDir)
	if err != nil {
		log.Fatalf("export requirements Markdown: %v", err)
	}
	fmt.Printf("requirements Markdown generated: %s\n", markdownPath)
}

func buildRunner(ctx context.Context, store ports.Store, cfg config.Config) (*pipeline.Runner, error) {
	aiProvider, err := providers.NewOpenAIProvider(cfg.OpenAIAPIKey)
	if err != nil {
		return nil, err
	}

	defaultPrompts := prompts.Defaults(time.Now().UTC())
	for _, prompt := range defaultPrompts {
		if _, err := store.CreatePrompt(ctx, prompt); err != nil {
			return nil, err
		}
	}

	return pipeline.NewRunner(store, stages.DefaultStages(aiProvider, aiProvider, prompts.Registry(defaultPrompts), stages.Models{
		Transcription: cfg.TranscriptionModel,
		Text:          cfg.TextModel,
	})), nil
}
func exportRun(ctx context.Context, store interface {
	ReconstructRun(context.Context, string) (domain.AuditRun, error)
}, runID string, outputDir string) (string, error) {
	audit, err := store.ReconstructRun(ctx, runID)
	if err != nil {
		return "", err
	}
	return export.WriteRequirementsMarkdown(outputDir, audit)
}
