package secrets

import "context"

type SecretFetcher interface {
	FetchSecrets(ctx context.Context, envName string) (map[string]string, error)
}
