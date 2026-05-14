package gc

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/eznix86/docker-registry-cleaner/internal/config"
)

type DockerRunner struct {
	cli *client.Client
	cfg config.Docker
}

func NewDocker(cfg config.Docker) (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	return &DockerRunner{
		cli: cli,
		cfg: cfg,
	}, nil
}

func (r *DockerRunner) RunGC(ctx context.Context) error {
	args := []string{"registry", "garbage-collect", r.cfg.GCConfigPath}
	if r.cfg.GCDeleteUnreferencedBlobs {
		args = append(args, "--delete-unreferenced-blobs")
	}

	execConfig := container.ExecOptions{
		User:         "",
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          args,
	}

	execID, err := r.cli.ContainerExecCreate(ctx, r.cfg.Container, execConfig)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	resp, err := r.cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("starting exec: %w", err)
	}
	defer resp.Close()

	_, err = io.Copy(io.Discard, resp.Reader)
	if err != nil {
		return fmt.Errorf("reading exec output: %w", err)
	}

	inspect, err := r.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("inspecting exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return fmt.Errorf("gc exited with code %d", inspect.ExitCode)
	}

	return nil
}

func (r *DockerRunner) DeleteEmptyRepositories(ctx context.Context, repos []string) error {
	if len(repos) == 0 {
		return nil
	}

	storagePath := r.cfg.StoragePath
	if storagePath == "" {
		storagePath = "/var/lib/registry"
	}

	for _, repo := range repos {
		repoPath := fmt.Sprintf("%s/docker/registry/v2/repositories/%s", storagePath, repo)
		command := []string{"rm", "-rf", repoPath}

		execConfig := container.ExecOptions{
			Tty:          false,
			AttachStdout: false,
			AttachStderr: false,
			Cmd:          command,
		}

		execID, err := r.cli.ContainerExecCreate(ctx, r.cfg.Container, execConfig)
		if err != nil {
			return fmt.Errorf("creating exec for %s: %w", repo, err)
		}

		if err := r.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			return fmt.Errorf("starting exec for %s: %w", repo, err)
		}

		inspect, err := r.cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return fmt.Errorf("inspecting exec for %s: %w", repo, err)
		}

		if inspect.ExitCode != 0 {
			return fmt.Errorf("deleting %s exited with code %d", repo, inspect.ExitCode)
		}
	}

	return nil
}
