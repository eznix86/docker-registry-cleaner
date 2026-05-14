package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/eznix86/docker-registry-cleaner/internal/config"
	"github.com/eznix86/docker-registry-cleaner/internal/gc"
	"github.com/eznix86/docker-registry-cleaner/internal/logger"
	"github.com/eznix86/docker-registry-cleaner/internal/registry"
)

func main() {
	configPath := flag.String("c", "", "path to config file")
	flag.Parse()

	if *configPath == "" {
		slog.Error("config file path is required, use -c flag")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Concurrency)

	for _, reg := range cfg.Registries {
		reg := reg
		regLogger := log.With("registry", reg.Name)

		g.Go(func() error {
			regLogger.Info("processing registry")

			client, err := registry.New(reg, cfg.Timeout, log)
			if err != nil {
				return err
			}

			if err := registry.CleanupRegistry(ctx, client, reg, cfg.DryRun, regLogger); err != nil {
				return err
			}

			if !cfg.DryRun {
				if err := gc.Run(ctx, reg, regLogger); err != nil {
					return err
				}
			} else {
				regLogger.Info("skipping garbage collection (dry run)")
			}

			regLogger.Info("registry processing complete")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error("cleanup failed", "error", err)
		os.Exit(1)
	}

	log.Info("all registries processed successfully")
}
