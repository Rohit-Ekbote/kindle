package iapssh_test

import (
	"testing"

	"github.com/emdash/kindle/internal/iapssh"
	"github.com/stretchr/testify/assert"
)

func TestParseTargetZone(t *testing.T) {
	zone, region := iapssh.ParseZoneAndRegion("us-central1-a")
	assert.Equal(t, "us-central1-a", zone)
	assert.Equal(t, "us-central1", region)
}

func TestNewClient_ConfigNotNil(t *testing.T) {
	cfg := iapssh.Config{
		Project: "my-project",
		Zone:    "us-central1-a",
	}
	client := iapssh.NewClient(cfg)
	assert.NotNil(t, client)
}
