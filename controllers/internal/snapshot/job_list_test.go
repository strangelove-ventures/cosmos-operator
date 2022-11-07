package snapshot

import (
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
)

func TestJobList_Add(t *testing.T) {
	var list JobList
	list.Add(batchv1.JobStatus{})

	require.Len(t, list.ToSlice(), 1)

	for i := 0; i < 15; i++ {
		list.Add(batchv1.JobStatus{})
	}

	require.Len(t, list.ToSlice(), 5)

	status := batchv1.JobStatus{Active: 1}
	list.Add(status)

	require.Equal(t, status, list.ToSlice()[0])
}

func TestJobList_Update(t *testing.T) {
	var list JobList
	list.Update(batchv1.JobStatus{})

	require.Empty(t, list.ToSlice())

	list.Add(batchv1.JobStatus{})
	list.Add(batchv1.JobStatus{})

	status := batchv1.JobStatus{Active: 1}
	list.Update(status)

	require.Len(t, list.ToSlice(), 2)
	require.Equal(t, status, list.ToSlice()[0])
}
