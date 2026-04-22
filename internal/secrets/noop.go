package secrets

import "context"

type NoOpFetcher struct{}

func NewNoOpFetcher() *NoOpFetcher {
	return &NoOpFetcher{}
}

func (f *NoOpFetcher) FetchSecrets(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}
