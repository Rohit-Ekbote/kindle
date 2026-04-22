package secrets

import (
	"context"
	"fmt"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type GCPFetcher struct {
	project string
	secrets []string
}

func NewGCPFetcher(secrets []string) (*GCPFetcher, error) {
	project := os.Getenv("GCP_PROJECT_ID")
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID is required for gcp backend")
	}
	return &GCPFetcher{project: project, secrets: secrets}, nil
}

func (f *GCPFetcher) FetchSecrets(ctx context.Context, _ string) (map[string]string, error) {
	if len(f.secrets) == 0 {
		return map[string]string{}, nil
	}
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create secret manager client: %w", err)
	}
	defer client.Close()

	result := make(map[string]string, len(f.secrets))
	for _, name := range f.secrets {
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", f.project, name),
		}
		resp, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("fetch secret %q: %w", name, err)
		}
		result[name] = string(resp.Payload.Data)
	}
	return result, nil
}
