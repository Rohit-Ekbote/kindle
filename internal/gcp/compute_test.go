package gcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/emdash/kindle/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVMClient implements gcp.VMClient for tests.
type fakeVMClient struct {
	vms []*gcp.VMInfo
}

func (f *fakeVMClient) ListEnvVMs(ctx context.Context) ([]*gcp.VMInfo, error) {
	return f.vms, nil
}

func (f *fakeVMClient) GetVM(ctx context.Context, name string) (*gcp.VMInfo, error) {
	for _, vm := range f.vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return nil, gcp.ErrVMNotFound
}

func TestFakeClient_ListEnvVMs(t *testing.T) {
	client := &fakeVMClient{
		vms: []*gcp.VMInfo{
			{Name: "dev-alice", Owner: "alice@example.com", MachineType: "e2-standard-2", Zone: "us-central1-a", ExternalIP: "1.2.3.4", CreatedAt: time.Now()},
		},
	}
	vms, err := client.ListEnvVMs(context.Background())
	require.NoError(t, err)
	assert.Len(t, vms, 1)
	assert.Equal(t, "dev-alice", vms[0].Name)
}

func TestFakeClient_GetVM_NotFound(t *testing.T) {
	client := &fakeVMClient{vms: nil}
	_, err := client.GetVM(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, gcp.ErrVMNotFound)
}
