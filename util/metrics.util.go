package util

import (
	"github.com/MishraShardendu22/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var MetricsRegistry = &model.Metrics{
	ReposProcessedTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "repos_processed_total",
		Help: "Total number of repositories processed",
	}),

	CloneFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "clone_failures_total",
		Help: "Total number of clone failures",
	}),

	BuildFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "build_failures_total",
		Help: "Total number of build failures",
	}),

	CleanupFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "cleanup_failures_total",
		Help: "Total number of cleanup failures",
	}),

	GitCommitFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "git_commit_failures_total",
		Help: "Total number of git commit failures",
	}),

	FilesDeletedTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "files_deleted_total",
		Help: "Total number of files deleted",
	}),

	ReactReposTotal: promauto.NewCounter(prometheus.CounterOpts{
		Name: "react_repos_total",
		Help: "Total number of React repositories found",
	}),

	ActiveWorkers: promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_workers",
		Help: "Current number of active worker goroutines",
	}),

	RepoProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "repo_processing_duration_seconds",
		Help:    "Duration of repository processing in seconds",
		Buckets: prometheus.DefBuckets,
	}),

	CloneDurationSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "clone_duration_seconds",
		Help:    "Duration of clone operations in seconds",
		Buckets: prometheus.DefBuckets,
	}),

	BuildDurationSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "build_duration_seconds",
		Help:    "Duration of build operations in seconds",
		Buckets: prometheus.DefBuckets,
	}),
}
