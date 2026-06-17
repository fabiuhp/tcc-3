package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	clearConfigEnv(t)

	cfg := Load()

	if cfg.MongoURI != DefaultMongoURI {
		t.Fatalf("MongoURI = %q, want %q", cfg.MongoURI, DefaultMongoURI)
	}
	if cfg.MongoDatabase != DefaultMongoDatabase {
		t.Fatalf("MongoDatabase = %q, want %q", cfg.MongoDatabase, DefaultMongoDatabase)
	}
	if cfg.TranscriptionModel != DefaultTranscriptionModel {
		t.Fatalf("TranscriptionModel = %q, want %q", cfg.TranscriptionModel, DefaultTranscriptionModel)
	}
	if cfg.TextModel != DefaultTextModel {
		t.Fatalf("TextModel = %q, want %q", cfg.TextModel, DefaultTextModel)
	}
	if cfg.DefaultLanguage != DefaultLanguage {
		t.Fatalf("DefaultLanguage = %q, want %q", cfg.DefaultLanguage, DefaultLanguage)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MONGO_URI", "mongodb://example:27017")
	t.Setenv("MONGO_DATABASE", "custom_db")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_TRANSCRIPTION_MODEL", "transcribe-test")
	t.Setenv("OPENAI_TEXT_MODEL", "text-test")
	t.Setenv("PIPELINE_DEFAULT_LANGUAGE", "en-US")

	cfg := Load()

	if cfg.MongoURI != "mongodb://example:27017" {
		t.Fatalf("MongoURI = %q", cfg.MongoURI)
	}
	if cfg.MongoDatabase != "custom_db" {
		t.Fatalf("MongoDatabase = %q", cfg.MongoDatabase)
	}
	if cfg.OpenAIAPIKey != "test-key" {
		t.Fatalf("OpenAIAPIKey = %q", cfg.OpenAIAPIKey)
	}
	if cfg.TranscriptionModel != "transcribe-test" {
		t.Fatalf("TranscriptionModel = %q", cfg.TranscriptionModel)
	}
	if cfg.TextModel != "text-test" {
		t.Fatalf("TextModel = %q", cfg.TextModel)
	}
	if cfg.DefaultLanguage != "en-US" {
		t.Fatalf("DefaultLanguage = %q", cfg.DefaultLanguage)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MONGO_URI",
		"MONGO_DATABASE",
		"OPENAI_API_KEY",
		"OPENAI_TRANSCRIPTION_MODEL",
		"OPENAI_TEXT_MODEL",
		"PIPELINE_DEFAULT_LANGUAGE",
	} {
		t.Setenv(key, "")
	}
}
