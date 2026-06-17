# AGENTS.md

## Commands
- Use Go 1.26+; the module is `requirement-pipeline` and there is no Makefile, CI workflow, or linter config in the repo.
- Verify with `go test ./...`; tests use mock AI providers and in-memory stores, so they do not need OpenAI or MongoDB.
- Run a focused test with `go test ./internal/<package> -run TestName`, for example `go test ./internal/export -run TestParseBusinessDiagrams`.
- Run the CLI with real services: `go run ./cmd/requirement-pipeline -title "Reuniao de descoberta" -audio ./meeting.mp3 -language pt-BR`.
- Export an existing run without reprocessing OpenAI: `go run ./cmd/requirement-pipeline -export-run-id "<pipeline-run-id>"`.
- Start web upload mode: `go run ./cmd/requirement-pipeline -web -addr :8080`.
- Format Go edits with `gofmt`; no separate formatter config is present.

## Runtime Requirements
- `config.Load()` auto-loads `.env`, then environment variables override it.
- Real CLI/web runs require MongoDB and `OPENAI_API_KEY`; defaults are `MONGO_URI=mongodb://localhost:27017`, `MONGO_DATABASE=requirement_pipeline`, `OPENAI_TRANSCRIPTION_MODEL=gpt-4o-transcribe`, `OPENAI_TEXT_MODEL=gpt-5`, and `PIPELINE_DEFAULT_LANGUAGE=pt-BR`.
- PDF diagram rendering shells out to `npx -y @mermaid-js/mermaid-cli`; if Chrome headless is missing, install it with `npx -y puppeteer browsers install chrome-headless-shell` or set `PUPPETEER_EXECUTABLE_PATH`.
- Runtime/generated paths are intentionally local: `.env`, `openspec/`, `.opencode/`, `.codex/`, `inicial.md`, and `output/` are ignored by `.gitignore`; web uploads default to `uploads/` and are deleted after processing.

## Architecture Notes
- The only app entrypoint is `cmd/requirement-pipeline/main.go`; it wires config, Mongo, OpenAI, prompt registration, pipeline stages, and PDF export.
- `internal/pipeline.Runner` is the ordered executor and audit writer; stages append artifacts, and `MustFindArtifact` searches from newest to oldest.
- Keep stage names and artifact types centralized in `internal/domain/models.go`; Mongo collection names are hard-coded in `internal/repository/mongo.go`.
- The default stage order is defined in `internal/stages/stages.go`: `audio_transcription`, `requirement_extraction`, `requirement_review`, `gap_analysis`, `requirement_refinement`, `diagram_generation`, `artifact_generation`.
- PDF export depends on `refined_requirements`; `business_diagrams` is optional and falls back gracefully if absent, invalid, or not renderable.
- Web assets are embedded by `//go:embed templates/*.html static/*.css` in `internal/web/server.go`; keep templates/static files under `internal/web/`.

## Workflow Notes
- When changing prompts, update `internal/prompts/prompts.go`; default prompts are inserted into Mongo every time a runner is built, with version `v1` unless changed there.
- Reprocessing with `-meeting-id` creates a new `pipeline_runs` record and preserves prior runs, stages, artifacts, and prompts.
- OpenSpec artifacts exist under `openspec/` with archived changes; use the repo-local OpenSpec workflow only when the task is explicitly spec/change oriented.
