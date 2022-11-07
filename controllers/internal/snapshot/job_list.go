package snapshot

import batchv1 "k8s.io/api/batch/v1"

type JobList struct {
	list []batchv1.JobStatus
}

// Add adds the status to the front of the list.
func (list *JobList) Add(status batchv1.JobStatus) {
	list.list = append([]batchv1.JobStatus{status}, list.list...)
	const maxSize = 5
	if len(list.list) > maxSize {
		list.list = list.list[:maxSize]
	}
}

// Update updates the most recent status.
// If the list is empty, this operation is a no-op.
func (list *JobList) Update(status batchv1.JobStatus) {
	if len(list.list) == 0 {
		return
	}
	list.list[0] = status
}

// ToSlice returns a slice of job statuses.
func (list *JobList) ToSlice() []batchv1.JobStatus { return list.list }
