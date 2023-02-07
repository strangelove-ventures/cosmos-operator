package statefuljob

import batchv1 "k8s.io/api/batch/v1"

// AddJobStatus adds the status to the head of the list.
func AddJobStatus(existing []batchv1.JobStatus, status batchv1.JobStatus) []batchv1.JobStatus {
	list := append([]batchv1.JobStatus{status}, existing...)
	const maxSize = 5
	if len(list) > maxSize {
		list = list[:maxSize]
	}
	return list
}

// UpdateJobStatus updates the most recent status (at the head).
// If the list is empty, this operation is a no-op.
func UpdateJobStatus(existing []batchv1.JobStatus, status batchv1.JobStatus) []batchv1.JobStatus {
	if len(existing) == 0 {
		return existing
	}
	return append([]batchv1.JobStatus{status}, existing[1:]...)
}
