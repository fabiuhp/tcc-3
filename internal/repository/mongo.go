package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/fabiuhp/tcc-3/internal/domain"
)

type MongoStore struct {
	db *mongo.Database
}

func ConnectMongo(ctx context.Context, uri, database string) (*mongo.Client, *MongoStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, nil, err
	}
	return client, &MongoStore{db: client.Database(database)}, nil
}

func (s *MongoStore) CreateMeeting(ctx context.Context, meeting domain.Meeting) (domain.Meeting, error) {
	if meeting.ID == "" {
		meeting.ID = domain.NewID()
	}
	if meeting.CreatedAt.IsZero() {
		meeting.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Collection("meetings").InsertOne(ctx, meeting)
	return meeting, err
}

func (s *MongoStore) GetMeeting(ctx context.Context, id string) (domain.Meeting, error) {
	var meeting domain.Meeting
	err := s.db.Collection("meetings").FindOne(ctx, bson.M{"_id": id}).Decode(&meeting)
	return meeting, err
}

func (s *MongoStore) CreatePipelineRun(ctx context.Context, run domain.PipelineRun) (domain.PipelineRun, error) {
	if run.ID == "" {
		run.ID = domain.NewID()
	}
	_, err := s.db.Collection("pipeline_runs").InsertOne(ctx, run)
	return run, err
}

func (s *MongoStore) UpdatePipelineRun(ctx context.Context, run domain.PipelineRun) error {
	_, err := s.db.Collection("pipeline_runs").ReplaceOne(ctx, bson.M{"_id": run.ID}, run)
	return err
}

func (s *MongoStore) GetPipelineRun(ctx context.Context, id string) (domain.PipelineRun, error) {
	var run domain.PipelineRun
	err := s.db.Collection("pipeline_runs").FindOne(ctx, bson.M{"_id": id}).Decode(&run)
	return run, err
}

func (s *MongoStore) ListPipelineRunsByMeeting(ctx context.Context, meetingID string) ([]domain.PipelineRun, error) {
	cur, err := s.db.Collection("pipeline_runs").Find(ctx, bson.M{"meeting_id": meetingID}, options.Find().SetSort(bson.D{{Key: "started_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var runs []domain.PipelineRun
	return runs, cur.All(ctx, &runs)
}

func (s *MongoStore) CreateStage(ctx context.Context, stage domain.StageExecution) (domain.StageExecution, error) {
	if stage.ID == "" {
		stage.ID = domain.NewID()
	}
	_, err := s.db.Collection("stages").InsertOne(ctx, stage)
	return stage, err
}

func (s *MongoStore) UpdateStage(ctx context.Context, stage domain.StageExecution) error {
	_, err := s.db.Collection("stages").ReplaceOne(ctx, bson.M{"_id": stage.ID}, stage)
	return err
}

func (s *MongoStore) ListStagesByRun(ctx context.Context, runID string) ([]domain.StageExecution, error) {
	cur, err := s.db.Collection("stages").Find(ctx, bson.M{"run_id": runID}, options.Find().SetSort(bson.D{{Key: "started_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var stages []domain.StageExecution
	return stages, cur.All(ctx, &stages)
}

func (s *MongoStore) CreateArtifact(ctx context.Context, artifact domain.Artifact) (domain.Artifact, error) {
	if artifact.ID == "" {
		artifact.ID = domain.NewID()
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Collection("artifacts").InsertOne(ctx, artifact)
	return artifact, err
}

func (s *MongoStore) CreateArtifacts(ctx context.Context, artifacts []domain.Artifact) ([]domain.Artifact, error) {
	created := make([]domain.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		item, err := s.CreateArtifact(ctx, artifact)
		if err != nil {
			return nil, err
		}
		created = append(created, item)
	}
	return created, nil
}

func (s *MongoStore) GetArtifact(ctx context.Context, id string) (domain.Artifact, error) {
	var artifact domain.Artifact
	err := s.db.Collection("artifacts").FindOne(ctx, bson.M{"_id": id}).Decode(&artifact)
	return artifact, err
}

func (s *MongoStore) ListArtifactsByStage(ctx context.Context, stageID string) ([]domain.Artifact, error) {
	cur, err := s.db.Collection("artifacts").Find(ctx, bson.M{"stage_id": stageID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var artifacts []domain.Artifact
	return artifacts, cur.All(ctx, &artifacts)
}

func (s *MongoStore) CreatePrompt(ctx context.Context, prompt domain.Prompt) (domain.Prompt, error) {
	if prompt.ID == "" {
		prompt.ID = domain.NewID()
	}
	if prompt.CreatedAt.IsZero() {
		prompt.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Collection("prompts").InsertOne(ctx, prompt)
	return prompt, err
}

func (s *MongoStore) GetLatestPromptByStage(ctx context.Context, stageName domain.StageName) (domain.Prompt, error) {
	var prompt domain.Prompt
	err := s.db.Collection("prompts").FindOne(ctx, bson.M{"stage_name": stageName}, options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})).Decode(&prompt)
	return prompt, err
}

func (s *MongoStore) ListPrompts(ctx context.Context) ([]domain.Prompt, error) {
	cur, err := s.db.Collection("prompts").Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var prompts []domain.Prompt
	return prompts, cur.All(ctx, &prompts)
}

func (s *MongoStore) ReconstructRun(ctx context.Context, runID string) (domain.AuditRun, error) {
	run, err := s.GetPipelineRun(ctx, runID)
	if err != nil {
		return domain.AuditRun{}, err
	}
	meeting, err := s.GetMeeting(ctx, run.MeetingID)
	if err != nil {
		return domain.AuditRun{}, err
	}
	stages, err := s.ListStagesByRun(ctx, run.ID)
	if err != nil {
		return domain.AuditRun{}, err
	}
	artifacts := []domain.Artifact{}
	for _, stage := range stages {
		stageArtifacts, err := s.ListArtifactsByStage(ctx, stage.ID)
		if err != nil {
			return domain.AuditRun{}, err
		}
		artifacts = append(artifacts, stageArtifacts...)
	}
	prompts, err := s.ListPrompts(ctx)
	if err != nil {
		return domain.AuditRun{}, err
	}
	return domain.AuditRun{Meeting: meeting, Run: run, Stages: stages, Artifacts: artifacts, Prompts: prompts}, nil
}
