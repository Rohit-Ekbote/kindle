package terraform_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/emdash/kindle/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobMap_SetAndGet(t *testing.T) {
	jm := terraform.NewJobMap()
	jm.Set("env-alice", terraform.JobStatus{State: terraform.StateProvisioning, Actor: "alice@example.com"})

	job, ok := jm.Get("env-alice")
	require.True(t, ok)
	assert.Equal(t, terraform.StateProvisioning, job.State)
	assert.Equal(t, "alice@example.com", job.Actor)
}

func TestJobMap_Delete(t *testing.T) {
	jm := terraform.NewJobMap()
	jm.Set("env-bob", terraform.JobStatus{State: terraform.StateProvisioning})
	jm.Delete("env-bob")
	_, ok := jm.Get("env-bob")
	assert.False(t, ok)
}

func TestJobMap_ConcurrentAccess(t *testing.T) {
	jm := terraform.NewJobMap()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("env-%d", i)
			jm.Set(name, terraform.JobStatus{State: terraform.StateProvisioning, StartedAt: time.Now()})
			jm.Get(name)
			jm.Delete(name)
		}(i)
	}
	wg.Wait()
}
