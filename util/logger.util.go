package util

import (
	"log/slog"
	"os"
	"time"
)

func attrsToAny(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}

func NewLogger() *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}).WithAttrs([]slog.Attr{
			slog.String("service", "github-cleaner"),
		}),
	)
}

func LogCloneStart(logger *slog.Logger, repo string) {
	logger.Info("clone_start",
		slog.String("repo", repo),
		slog.String("operation", "clone"),
	)
}

func LogCloneEnd(logger *slog.Logger, repo string, duration time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("repo", repo),
		slog.String("operation", "clone"),
		slog.Duration("duration", duration),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("clone_end", attrsToAny(attrs)...)
	} else {
		logger.Info("clone_end", attrsToAny(attrs)...)
	}
}

func LogCleanupStart(logger *slog.Logger, repo string) {
	logger.Info("cleanup_start",
		slog.String("repo", repo),
		slog.String("operation", "cleanup"),
	)
}

func LogCleanupEnd(logger *slog.Logger, repo string, duration time.Duration, deletedFiles int, err error) {
	attrs := []slog.Attr{
		slog.String("repo", repo),
		slog.String("operation", "cleanup"),
		slog.Duration("duration", duration),
		slog.Int("deleted_files", deletedFiles),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("cleanup_end", attrsToAny(attrs)...)
	} else {
		logger.Info("cleanup_end", attrsToAny(attrs)...)
	}
}

func LogBuildStart(logger *slog.Logger, repo string) {
	logger.Info("build_start",
		slog.String("repo", repo),
		slog.String("operation", "build"),
	)
}

func LogBuildEnd(logger *slog.Logger, repo string, duration time.Duration, status string, err error) {
	attrs := []slog.Attr{
		slog.String("repo", repo),
		slog.String("operation", "build"),
		slog.Duration("duration", duration),
		slog.String("build_status", status),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("build_end", attrsToAny(attrs)...)
	} else {
		logger.Info("build_end", attrsToAny(attrs)...)
	}
}

func LogGitCommitStart(logger *slog.Logger, repo string) {
	logger.Info("git_commit_start",
		slog.String("repo", repo),
		slog.String("operation", "git_commit"),
	)
}

func LogGitCommitEnd(logger *slog.Logger, repo string, duration time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("repo", repo),
		slog.String("operation", "git_commit"),
		slog.Duration("duration", duration),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("git_commit_end", attrsToAny(attrs)...)
	} else {
		logger.Info("git_commit_end", attrsToAny(attrs)...)
	}
}

func LogRepoComplete(logger *slog.Logger, repo string, totalDuration time.Duration, deletedFiles int, buildStatus string) {
	logger.Info("repo_complete",
		slog.String("repo", repo),
		slog.String("operation", "repo_complete"),
		slog.Duration("duration", totalDuration),
		slog.Int("deleted_files", deletedFiles),
		slog.String("build_status", buildStatus),
	)
}

func LogFailure(logger *slog.Logger, repo string, operation string, err error) {
	logger.Error("failure",
		slog.String("repo", repo),
		slog.String("operation", operation),
		slog.String("error", err.Error()),
	)
}