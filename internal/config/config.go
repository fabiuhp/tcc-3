package config

import (
	"cmp"
	"os"

	"github.com/joho/godotenv"
)

const (
	DefaultMongoURI           = "mongodb://localhost:27017"
	DefaultMongoDatabase      = "requirement_pipeline"
	DefaultTranscriptionModel = "gpt-4o-transcribe"
	DefaultTextModel          = "gpt-5"
	DefaultLanguage           = "pt-BR"
)

type Config struct {
	MongoURI           string
	MongoDatabase      string
	OpenAIAPIKey       string
	TranscriptionModel string
	TextModel          string
	DefaultLanguage    string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		MongoURI:           cmp.Or(os.Getenv("MONGO_URI"), DefaultMongoURI),
		MongoDatabase:      cmp.Or(os.Getenv("MONGO_DATABASE"), DefaultMongoDatabase),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		TranscriptionModel: cmp.Or(os.Getenv("OPENAI_TRANSCRIPTION_MODEL"), DefaultTranscriptionModel),
		TextModel:          cmp.Or(os.Getenv("OPENAI_TEXT_MODEL"), DefaultTextModel),
		DefaultLanguage:    cmp.Or(os.Getenv("PIPELINE_DEFAULT_LANGUAGE"), DefaultLanguage),
	}
}
