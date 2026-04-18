package k8s_test

import (
	"context"
	"testing"

	"github.com/emdash/kindle/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkloadClient struct {
	workloads []k8s.Workload
}

func (f *fakeWorkloadClient) ListWorkloads(ctx context.Context, namespace string) ([]k8s.Workload, error) {
	return f.workloads, nil
}

func TestFakeClient_ListWorkloads(t *testing.T) {
	client := &fakeWorkloadClient{
		workloads: []k8s.Workload{
			{Kind: "Deployment", Name: "api", Namespace: "dev-alice", ReadyReplicas: 1, DesiredReplicas: 1,
				Containers: []k8s.ContainerInfo{{Name: "api", Image: "myrepo/api:v1.2.3"}}},
		},
	}
	workloads, err := client.ListWorkloads(context.Background(), "dev-alice")
	require.NoError(t, err)
	assert.Len(t, workloads, 1)
	assert.Equal(t, "Deployment", workloads[0].Kind)
	assert.Equal(t, "myrepo/api:v1.2.3", workloads[0].Containers[0].Image)
}

func TestParseImageTag(t *testing.T) {
	repo, tag := k8s.ParseImageTag("myrepo/api:v1.2.3")
	assert.Equal(t, "myrepo/api", repo)
	assert.Equal(t, "v1.2.3", tag)
}

func TestParseImageTag_NoTag(t *testing.T) {
	repo, tag := k8s.ParseImageTag("myrepo/api")
	assert.Equal(t, "myrepo/api", repo)
	assert.Equal(t, "latest", tag)
}
