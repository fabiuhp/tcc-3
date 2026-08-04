package repository

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/fabiuhp/tcc-3/internal/domain"
)

var ErrNotFound = errors.New("not found")

type MemoryStore struct {
	mu        sync.RWMutex
	meetings  map[string]domain.Meeting
	runs      map[string]domain.PipelineRun
	stages    map[string]domain.StageExecution
	artifacts map[string]domain.Artifact
	prompts   map[string]domain.Prompt
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		meetings:  map[string]domain.Meeting{},
		runs:      map[string]domain.PipelineRun{},
		stages:    map[string]domain.StageExecution{},
		artifacts: map[string]domain.Artifact{},
		prompts:   map[string]domain.Prompt{},
	}
}

func (s *MemoryStore) CreateMeeting(_ context.Context, meeting domain.Meeting) (domain.Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if meeting.ID == "" {
		meeting.ID = domain.NewID()
	}
	if meeting.CreatedAt.IsZero() {
		meeting.CreatedAt = time.Now().UTC()
	}
	s.meetings[meeting.ID] = meeting
	return meeting, nil
}

func (s *MemoryStore) GetMeeting(_ context.Context, id string) (domain.Meeting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meeting, ok := s.meetings[id]
	if !ok {
		return domain.Meeting{}, ErrNotFound
	}
	return meeting, nil
}

func (s *MemoryStore) CreatePipelineRun(_ context.Context, run domain.PipelineRun) (domain.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run.ID == "" {
		run.ID = domain.NewID()
	}
	s.runs[run.ID] = run
	return run, nil
}

func (s *MemoryStore) UpdatePipelineRun(_ context.Context, run domain.PipelineRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[run.ID]; !ok {
		return ErrNotFound
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetPipelineRun(_ context.Context, id string) (domain.PipelineRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return domain.PipelineRun{}, ErrNotFound
	}
	return run, nil
}

func (s *MemoryStore) ListPipelineRunsByMeeting(_ context.Context, meetingID string) ([]domain.PipelineRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := []domain.PipelineRun{}
	for _, run := range s.runs {
		if run.MeetingID == meetingID {
			runs = append(runs, run)
		}
	}
	slices.SortFunc(runs, func(a, b domain.PipelineRun) int { return a.StartedAt.Compare(b.StartedAt) })
	return runs, nil
}

func (s *MemoryStore) CreateStage(_ context.Context, stage domain.StageExecution) (domain.StageExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stage.ID == "" {
		stage.ID = domain.NewID()
	}
	s.stages[stage.ID] = stage
	return stage, nil
}

func (s *MemoryStore) UpdateStage(_ context.Context, stage domain.StageExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stages[stage.ID]; !ok {
		return ErrNotFound
	}
	s.stages[stage.ID] = stage
	return nil
}

func (s *MemoryStore) ListStagesByRun(_ context.Context, runID string) ([]domain.StageExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stages := []domain.StageExecution{}
	for _, stage := range s.stages {
		if stage.RunID == runID {
			stages = append(stages, stage)
		}
	}
	slices.SortFunc(stages, func(a, b domain.StageExecution) int { return a.StartedAt.Compare(b.StartedAt) })
	return stages, nil
}

func (s *MemoryStore) CreateArtifact(_ context.Context, artifact domain.Artifact) (domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if artifact.ID == "" {
		artifact.ID = domain.NewID()
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}
	s.artifacts[artifact.ID] = artifact
	return artifact, nil
}

func (s *MemoryStore) CreateArtifacts(ctx context.Context, artifacts []domain.Artifact) ([]domain.Artifact, error) {
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

func (s *MemoryStore) GetArtifact(_ context.Context, id string) (domain.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.artifacts[id]
	if !ok {
		return domain.Artifact{}, ErrNotFound
	}
	return artifact, nil
}

func (s *MemoryStore) ListArtifactsByStage(_ context.Context, stageID string) ([]domain.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifacts := []domain.Artifact{}
	for _, artifact := range s.artifacts {
		if artifact.StageID == stageID {
			artifacts = append(artifacts, artifact)
		}
	}
	slices.SortFunc(artifacts, func(a, b domain.Artifact) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return artifacts, nil
}

func (s *MemoryStore) CreatePrompt(_ context.Context, prompt domain.Prompt) (domain.Prompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prompt.ID == "" {
		prompt.ID = domain.NewID()
	}
	if prompt.CreatedAt.IsZero() {
		prompt.CreatedAt = time.Now().UTC()
	}
	s.prompts[prompt.ID] = prompt
	return prompt, nil
}

func (s *MemoryStore) GetLatestPromptByStage(_ context.Context, stageName domain.StageName) (domain.Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest domain.Prompt
	for _, prompt := range s.prompts {
		if prompt.StageName == stageName && (latest.ID == "" || prompt.CreatedAt.After(latest.CreatedAt)) {
			latest = prompt
		}
	}
	if latest.ID == "" {
		return domain.Prompt{}, ErrNotFound
	}
	return latest, nil
}

func (s *MemoryStore) ListPrompts(_ context.Context) ([]domain.Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prompts := make([]domain.Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, prompt)
	}
	slices.SortFunc(prompts, func(a, b domain.Prompt) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return prompts, nil
}

func (s *MemoryStore) ReconstructRun(ctx context.Context, runID string) (domain.AuditRun, error) {
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
