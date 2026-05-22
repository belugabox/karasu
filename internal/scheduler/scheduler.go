package scheduler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

type Scheduler struct {
	gocronScheduler gocron.Scheduler
	jobStartTimes   map[uuid.UUID]time.Time
	mu              sync.Mutex
}

func NewScheduler() (Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return Scheduler{}, fmt.Errorf("failed to create scheduler: %w", err)
	}
	return Scheduler{
		gocronScheduler: s,
		jobStartTimes:   make(map[uuid.UUID]time.Time),
	}, nil
}

func (s *Scheduler) AddJob(jobName string, duration time.Duration, jobTask func() error) error {
	_, err := s.gocronScheduler.NewJob(
		gocron.DurationJob(duration),
		gocron.NewTask(jobTask),
		gocron.WithName(jobName),
		gocron.WithStartAt(gocron.WithStartImmediately()),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.BeforeJobRuns(func(jobID uuid.UUID, jobName string) {
				s.mu.Lock()
				s.jobStartTimes[jobID] = time.Now()
				s.mu.Unlock()
			}),
			gocron.AfterJobRuns(func(jobID uuid.UUID, jobName string) {
				slog.Debug("job completed", "name", jobName, "duration", s.jobDuration(jobID))
			}),
			gocron.AfterJobRunsWithError(func(jobID uuid.UUID, jobName string, err error) {
				slog.Error("job failed", "name", jobName, "duration", s.jobDuration(jobID), "err", err)
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to add job: %w", err)
	}
	return nil
}

func (s *Scheduler) Start() {
	s.gocronScheduler.Start()
}

func (s *Scheduler) Stop() {
	s.gocronScheduler.Shutdown()
}

func (s *Scheduler) jobDuration(jobID uuid.UUID) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	start, ok := s.jobStartTimes[jobID]
	if !ok {
		return 0
	}

	delete(s.jobStartTimes, jobID)
	return time.Since(start)
}
