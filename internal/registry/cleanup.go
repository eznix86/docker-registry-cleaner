package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"time"

	"github.com/eznix86/docker-registry-cleaner/internal/config"
	"github.com/eznix86/registry-client"
)

type TagInfo struct {
	Tag       string
	Digest    string
	CreatedAt time.Time
}

func CleanupRegistry(ctx context.Context, client *Client, cfg config.Registry, dryRun bool, logger *slog.Logger) error {
	logger.Info("starting registry cleanup", "url", cfg.URL)

	repos, err := client.GetCatalog(ctx)
	if err != nil {
		return fmt.Errorf("getting catalog: %w", err)
	}

	logger.Info("found repositories", "count", len(repos))

	for _, repo := range repos {
		if cfg.RepoSkip(repo) {
			logger.Info("skipping repository", "repo", repo)
			continue
		}

		if err := cleanupRepo(ctx, client, cfg, repo, dryRun, logger); err != nil {
			return fmt.Errorf("cleaning repo %s: %w", repo, err)
		}
	}

	return nil
}

func cleanupRepo(ctx context.Context, client *Client, cfg config.Registry, repo string, dryRun bool, logger *slog.Logger) error {
	repoLogger := logger.With("repo", repo)

	tags, err := client.ListTags(ctx, repo)
	if err != nil {
		return fmt.Errorf("listing tags: %w", err)
	}

	if len(tags) == 0 {
		repoLogger.Info("no tags found")
		return nil
	}

	tagFilter, err := cfg.RepoTagFilter(repo)
	if err != nil {
		return fmt.Errorf("compiling tag filter: %w", err)
	}

	tagInfos, err := fetchTagInfo(ctx, client, repo, tags, tagFilter, repoLogger)
	if err != nil {
		return fmt.Errorf("fetching tag info: %w", err)
	}

	keep := cfg.RepoKeep(repo)
	deleteUntagged := cfg.RepoDeleteUntagged(repo)

	toDelete := selectTagsToDelete(tagInfos, keep, deleteUntagged, repoLogger)

	if len(toDelete) == 0 {
		repoLogger.Info("nothing to delete")
		return nil
	}

	for _, tag := range toDelete {
		if dryRun {
			repoLogger.Info("would delete manifest", "tag", tag.Tag, "digest", tag.Digest)
			continue
		}

		repoLogger.Info("deleting manifest", "tag", tag.Tag, "digest", tag.Digest)
		if err := client.DeleteManifest(ctx, repo, tag.Digest); err != nil {
			return fmt.Errorf("deleting manifest %s: %w", tag.Digest, err)
		}
	}

	repoLogger.Info("cleanup complete", "deleted", len(toDelete), "kept", len(tagInfos)-len(toDelete))
	return nil
}

func fetchTagInfo(ctx context.Context, client *Client, repo string, tags []string, tagFilter *regexp.Regexp, logger *slog.Logger) ([]TagInfo, error) {
	var tagInfos []TagInfo

	for _, tag := range tags {
		if tagFilter != nil && !tagFilter.MatchString(tag) {
			logger.Debug("skipping tag (filter mismatch)", "tag", tag)
			continue
		}

		manifest, err := client.GetManifest(ctx, repo, tag)
		if err != nil {
			logger.Warn("failed to get manifest", "tag", tag, "error", err)
			continue
		}

		var createdAt time.Time

		if manifest.ManifestData != nil {
			switch m := manifest.ManifestData.(type) {
			case registryclient.ImageManifest:
				configBlob, err := client.GetBlob(ctx, repo, m.Config.Digest)
				if err != nil {
					logger.Warn("failed to get config blob", "tag", tag, "digest", m.Config.Digest, "error", err)
				} else {
					var cfg registryclient.ConfigBlob
					if err := json.Unmarshal(configBlob.Content, &cfg); err != nil {
						logger.Warn("failed to parse config blob", "tag", tag, "error", err)
					} else if cfg.Created != "" {
						createdAt, _ = time.Parse(time.RFC3339, cfg.Created)
					}
				}

			case registryclient.ManifestList:
				if len(m.Manifests) > 0 {
					firstManifest, err := client.GetManifest(ctx, repo, m.Manifests[0].Digest)
					if err != nil {
						logger.Warn("failed to get platform manifest", "tag", tag, "error", err)
					} else if img, ok := firstManifest.ManifestData.(registryclient.ImageManifest); ok {
						configBlob, err := client.GetBlob(ctx, repo, img.Config.Digest)
						if err != nil {
							logger.Warn("failed to get config blob", "tag", tag, "error", err)
						} else {
							var cfg registryclient.ConfigBlob
							if err := json.Unmarshal(configBlob.Content, &cfg); err != nil {
								logger.Warn("failed to parse config blob", "tag", tag, "error", err)
							} else if cfg.Created != "" {
								createdAt, _ = time.Parse(time.RFC3339, cfg.Created)
							}
						}
					}
				}
			}
		}

		tagInfos = append(tagInfos, TagInfo{
			Tag:       tag,
			Digest:    manifest.Digest,
			CreatedAt: createdAt,
		})
	}

	return tagInfos, nil
}

func selectTagsToDelete(tagInfos []TagInfo, keep int, deleteUntagged bool, logger *slog.Logger) []TagInfo {
	sort.Slice(tagInfos, func(i, j int) bool {
		if tagInfos[i].CreatedAt.IsZero() || tagInfos[j].CreatedAt.IsZero() {
			return tagInfos[i].Tag < tagInfos[j].Tag
		}
		return tagInfos[i].CreatedAt.After(tagInfos[j].CreatedAt)
	})

	var toDelete []TagInfo

	if keep < len(tagInfos) {
		toDelete = append(toDelete, tagInfos[keep:]...)
	}

	if deleteUntagged {
		for _, tag := range tagInfos {
			if tag.CreatedAt.IsZero() {
				found := false
				for _, d := range toDelete {
					if d.Digest == tag.Digest {
						found = true
						break
					}
				}
				if !found {
					toDelete = append(toDelete, tag)
				}
			}
		}
	}

	return toDelete
}
