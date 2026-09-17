package riveradapter

import (
	"time"

	"github.com/riverqueue/river"
)

const (
	// hard-coded interval for now
	Interval = 1 * time.Minute
)

func CreatePeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(Interval),
			func() (river.JobArgs, *river.InsertOpts) {
				return OutboxEventPublisherJobArgs{}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: true},
		),
	}
}
