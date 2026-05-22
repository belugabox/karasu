package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func newScheduler() (gocron.Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	/*_, err = s.NewJob(
		gocron.DurationJob(60*time.Second),
		gocron.NewTask(jobTask),
		gocron.WithName("ETL Task"),
		gocron.WithStartAt(gocron.WithStartImmediately()),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.AfterJobRuns(func(jobID uuid.UUID, jobName string) {
				slog.Debug("job completed", "name", jobName)
			}),
			gocron.AfterJobRunsWithError(func(jobID uuid.UUID, jobName string, err error) {
				slog.Error("job failed", "name", jobName, "err", err)
			}),
		),
	)
	if err != nil {
		if shutdownErr := s.Shutdown(); shutdownErr != nil {
			slog.Warn("failed to shutdown scheduler after job creation error", "err", shutdownErr)
		}
		return nil, fmt.Errorf("failed on job creation: %w", err)
	}*/

	return s, nil
}

func addJob(s gocron.Scheduler, jobName string, duration time.Duration, jobTask func() error) error {
	_, err := s.NewJob(
		gocron.DurationJob(duration),
		gocron.NewTask(jobTask),
		gocron.WithName(jobName),
		gocron.WithStartAt(gocron.WithStartImmediately()),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.AfterJobRuns(func(jobID uuid.UUID, jobName string) {
				slog.Debug("job completed", "name", jobName)
			}),
			gocron.AfterJobRunsWithError(func(jobID uuid.UUID, jobName string, err error) {
				slog.Error("job failed", "name", jobName, "err", err)
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to add job: %w", err)
	}
	return nil
}
