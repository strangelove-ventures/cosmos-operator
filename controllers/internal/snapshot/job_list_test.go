package snapshot

import (
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
)

func TestAddJobStatus(t *testing.T) {
	t.Parallel()

	list := AddJobStatus(nil, batchv1.JobStatus{})
	require.Len(t, list, 1)

	for i := 0; i < 15; i++ {
		list = AddJobStatus(list, batchv1.JobStatus{})
	}

	require.Len(t, list, 5)

	status := batchv1.JobStatus{Active: 1}
	list = AddJobStatus(list, status)

	require.Equal(t, status, list[0])
}

func TestJobList_Update(t *testing.T) {
	t.Parallel()

	list := UpdateJobStatus(nil, batchv1.JobStatus{})

	require.Empty(t, list)

	status := batchv1.JobStatus{Active: 1}
	list = UpdateJobStatus(make([]batchv1.JobStatus, 2), status)

	require.Len(t, list, 2)
	require.Equal(t, status, list[0])
}
