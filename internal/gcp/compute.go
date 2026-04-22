package gcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

var ErrVMNotFound = errors.New("VM not found")

type VMInfo struct {
	Name        string
	Owner       string
	Template    string
	MachineType string
	Zone        string
	ExternalIP  string
	InternalIP  string
	DiskSizeGB  int64
	CreatedAt   time.Time
	Status      string // GCP status: RUNNING, TERMINATED, etc.
}

type VMClient interface {
	ListEnvVMs(ctx context.Context) ([]*VMInfo, error)
	GetVM(ctx context.Context, name string) (*VMInfo, error)
}

type gcpVMClient struct {
	project string
	client  *compute.InstancesClient
}

func NewVMClient(ctx context.Context, project string) (VMClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}
	return &gcpVMClient{project: project, client: c}, nil
}

func (c *gcpVMClient) ListEnvVMs(ctx context.Context) ([]*VMInfo, error) {
	req := &computepb.AggregatedListInstancesRequest{
		Project: c.project,
		Filter:  strPtr("labels.managed-by=env-portal"),
	}
	var vms []*VMInfo
	it := c.client.AggregatedList(ctx, req)
	for {
		pair, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, inst := range pair.Value.Instances {
			vms = append(vms, instanceToVMInfo(inst))
		}
	}
	return vms, nil
}

func (c *gcpVMClient) GetVM(ctx context.Context, name string) (*VMInfo, error) {
	vms, err := c.ListEnvVMs(ctx)
	if err != nil {
		return nil, err
	}
	for _, vm := range vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return nil, ErrVMNotFound
}

func instanceToVMInfo(inst *computepb.Instance) *VMInfo {
	info := &VMInfo{
		Name:        inst.GetName(),
		MachineType: lastSegment(inst.GetMachineType()),
		Zone:        lastSegment(inst.GetZone()),
		Status:      inst.GetStatus(),
	}
	if inst.Labels != nil {
		info.Owner = inst.Labels["owner"]
		info.Template = inst.Labels["template"]
	}
	for _, ni := range inst.NetworkInterfaces {
		info.InternalIP = ni.GetNetworkIP()
		for _, ac := range ni.AccessConfigs {
			info.ExternalIP = ac.GetNatIP()
		}
	}
	for _, disk := range inst.Disks {
		if disk.GetBoot() {
			info.DiskSizeGB = disk.GetDiskSizeGb()
		}
	}
	t, _ := time.Parse(time.RFC3339, inst.GetCreationTimestamp())
	info.CreatedAt = t
	return info
}

func lastSegment(s string) string {
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

func strPtr(s string) *string { return &s }
