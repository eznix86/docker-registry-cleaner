package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogLevel    string        `yaml:"log_level"`
	DryRun      bool          `yaml:"dry_run"`
	Concurrency int           `yaml:"concurrency"`
	Timeout     time.Duration `yaml:"timeout"`
	Registries  []Registry    `yaml:"registries"`
}

type Registry struct {
	Name           string            `yaml:"name"`
	URL            string            `yaml:"url"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	UsernameEnv    string            `yaml:"username_env"`
	PasswordEnv    string            `yaml:"password_env"`
	Kubernetes     Kubernetes        `yaml:"kubernetes"`
	Docker         Docker            `yaml:"docker"`
	Keep           int               `yaml:"keep"`
	DeleteUntagged bool              `yaml:"delete_untagged"`
	Repos          map[string]Repo   `yaml:"repos"`
}

type Kubernetes struct {
	Enabled                   bool   `yaml:"enabled"`
	Namespace                 string `yaml:"namespace"`
	Workload                  string `yaml:"workload"`
	Name                      string `yaml:"name"`
	LabelSelector             string `yaml:"label_selector"`
	GCConfigPath              string `yaml:"gc_config_path"`
	GCDeleteUnreferencedBlobs bool   `yaml:"gc_delete_unreferenced_blobs"`
	DeleteEmptyRepos          bool   `yaml:"delete_empty_repos"`
	StoragePath               string `yaml:"storage_path"`
}

type Docker struct {
	Enabled                   bool   `yaml:"enabled"`
	Container                 string `yaml:"container"`
	GCConfigPath              string `yaml:"gc_config_path"`
	GCDeleteUnreferencedBlobs bool   `yaml:"gc_delete_unreferenced_blobs"`
}

type Repo struct {
	Keep           *int   `yaml:"keep"`
	TagFilter      string `yaml:"tag_filter"`
	DeleteUntagged *bool  `yaml:"delete_untagged"`
	Skip           bool   `yaml:"skip"`
}

func (r Registry) GetCredentials() (string, string, error) {
	username := r.Username
	password := r.Password

	if r.UsernameEnv != "" {
		username = os.Getenv(r.UsernameEnv)
		if username == "" {
			return "", "", fmt.Errorf("environment variable %s is not set", r.UsernameEnv)
		}
	}

	if r.PasswordEnv != "" {
		password = os.Getenv(r.PasswordEnv)
		if password == "" {
			return "", "", fmt.Errorf("environment variable %s is not set", r.PasswordEnv)
		}
	}

	return username, password, nil
}

func (r Registry) RepoConfig(repoName string) Repo {
	if repo, ok := r.Repos[repoName]; ok {
		return repo
	}
	return Repo{}
}

func (r Registry) RepoKeep(repoName string) int {
	repo := r.RepoConfig(repoName)
	if repo.Keep != nil {
		return *repo.Keep
	}
	return r.Keep
}

func (r Registry) RepoDeleteUntagged(repoName string) bool {
	repo := r.RepoConfig(repoName)
	if repo.DeleteUntagged != nil {
		return *repo.DeleteUntagged
	}
	return r.DeleteUntagged
}

func (r Registry) RepoTagFilter(repoName string) (*regexp.Regexp, error) {
	repo := r.RepoConfig(repoName)
	if repo.TagFilter == "" {
		return nil, nil
	}
	return regexp.Compile(repo.TagFilter)
}

func (r Registry) RepoSkip(repoName string) bool {
	repo := r.RepoConfig(repoName)
	return repo.Skip
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Concurrency <= 0 {
		c.Concurrency = 1
	}

	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if len(c.Registries) == 0 {
		return fmt.Errorf("no registries configured")
	}

	for i, reg := range c.Registries {
		if reg.Name == "" {
			return fmt.Errorf("registries[%d]: name is required", i)
		}
		if reg.URL == "" {
			return fmt.Errorf("registries[%d]: url is required", i)
		}
		if reg.Keep <= 0 {
			return fmt.Errorf("registries[%d]: keep must be greater than 0", i)
		}
		if reg.Kubernetes.Enabled {
			if reg.Kubernetes.Namespace == "" {
				return fmt.Errorf("registries[%d]: kubernetes.namespace is required", i)
			}
			if reg.Kubernetes.Name == "" && reg.Kubernetes.LabelSelector == "" {
				return fmt.Errorf("registries[%d]: kubernetes.name or kubernetes.label_selector is required", i)
			}
			if reg.Kubernetes.GCConfigPath == "" {
				return fmt.Errorf("registries[%d]: kubernetes.gc_config_path is required", i)
			}
		}
		if reg.Docker.Enabled {
			if reg.Docker.Container == "" {
				return fmt.Errorf("registries[%d]: docker.container is required", i)
			}
			if reg.Docker.GCConfigPath == "" {
				return fmt.Errorf("registries[%d]: docker.gc_config_path is required", i)
			}
		}
	}

	return nil
}
