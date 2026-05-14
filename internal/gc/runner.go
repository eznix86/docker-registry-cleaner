package gc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/eznix86/docker-registry-cleaner/internal/config"
)

type Runner interface {
	RunGC(ctx context.Context) error
}

func Run(ctx context.Context, cfg config.Registry, logger *slog.Logger) error {
	if cfg.Kubernetes.Enabled {
		logger.Info("running garbage collection via kubernetes",
			"namespace", cfg.Kubernetes.Namespace,
			"workload", cfg.Kubernetes.Workload,
			"name", cfg.Kubernetes.Name,
		)

		runner, err := NewKubernetes(cfg.Kubernetes)
		if err != nil {
			return fmt.Errorf("creating kubernetes runner: %w", err)
		}

		if err := runner.RunGC(ctx); err != nil {
			return fmt.Errorf("kubernetes gc: %w", err)
		}

		logger.Info("garbage collection completed via kubernetes")
		return nil
	}

	if cfg.Docker.Enabled {
		logger.Info("running garbage collection via docker",
			"container", cfg.Docker.Container,
		)

		runner, err := NewDocker(cfg.Docker)
		if err != nil {
			return fmt.Errorf("creating docker runner: %w", err)
		}

		if err := runner.RunGC(ctx); err != nil {
			return fmt.Errorf("docker gc: %w", err)
		}

		logger.Info("garbage collection completed via docker")
		return nil
	}

	logger.Info("skipping garbage collection (no kubernetes/docker block enabled)")
	return nil
}

func DeleteEmptyRepositories(ctx context.Context, kCfg config.Kubernetes, dCfg config.Docker, repos []string, logger *slog.Logger) error {
	if kCfg.Enabled && kCfg.DeleteEmptyRepos {
		runner, err := NewKubernetes(kCfg)
		if err != nil {
			return fmt.Errorf("creating kubernetes runner: %w", err)
		}

		if err := runner.DeleteEmptyRepositories(ctx, repos); err != nil {
			return fmt.Errorf("kubernetes delete empty repos: %w", err)
		}

		logger.Info("empty repositories deleted via kubernetes", "count", len(repos))
		return nil
	}

	if dCfg.Enabled && dCfg.DeleteEmptyRepos {
		runner, err := NewDocker(dCfg)
		if err != nil {
			return fmt.Errorf("creating docker runner: %w", err)
		}

		if err := runner.DeleteEmptyRepositories(ctx, repos); err != nil {
			return fmt.Errorf("docker delete empty repos: %w", err)
		}

		logger.Info("empty repositories deleted via docker", "count", len(repos))
		return nil
	}

	return nil
}
