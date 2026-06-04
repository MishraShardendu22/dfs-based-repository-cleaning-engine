package model

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	ReposProcessedTotal    prometheus.Counter
	CloneFailuresTotal     prometheus.Counter
	BuildFailuresTotal     prometheus.Counter
	CleanupFailuresTotal   prometheus.Counter
	GitCommitFailuresTotal prometheus.Counter
	FilesDeletedTotal      prometheus.Counter
	ReactReposTotal        prometheus.Counter

	ActiveWorkers prometheus.Gauge

	RepoProcessingDuration prometheus.Histogram
	CloneDurationSeconds   prometheus.Histogram
	BuildDurationSeconds   prometheus.Histogram
}
