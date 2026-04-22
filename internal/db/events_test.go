package db_test

import (
	"testing"

	"github.com/emdash/kindle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestWriteAndListEvents(t *testing.T) {
	d := openTestDB(t)

	err := d.WriteEvent(db.Event{
		EnvName:     "dev-alice",
		ActorEmail:  "alice@example.com",
		ActionType:  "create",
		Description: "Created env dev-alice from preset small-dev",
		Outcome:     db.OutcomeInProgress,
	})
	require.NoError(t, err)

	events, err := d.ListEvents("dev-alice", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "create", events[0].ActionType)
	assert.Equal(t, db.OutcomeInProgress, events[0].Outcome)
	assert.False(t, events[0].CreatedAt.IsZero())
}

func TestListEvents_Pagination(t *testing.T) {
	d := openTestDB(t)

	for i := 0; i < 5; i++ {
		require.NoError(t, d.WriteEvent(db.Event{
			EnvName:     "dev-alice",
			ActorEmail:  "alice@example.com",
			ActionType:  "create",
			Description: "event",
			Outcome:     db.OutcomeSuccess,
		}))
	}

	page1, err := d.ListEvents("dev-alice", 3, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 3)

	page2, err := d.ListEvents("dev-alice", 3, 3)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}

func TestListEvents_Empty(t *testing.T) {
	d := openTestDB(t)
	events, err := d.ListEvents("nonexistent", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestUpdateOutcome_NoMatch(t *testing.T) {
	d := openTestDB(t)
	err := d.UpdateOutcome("nonexistent", "create", db.OutcomeSuccess)
	assert.ErrorIs(t, err, db.ErrNotFound)
}

func TestWriteEvent_UpdateOutcome(t *testing.T) {
	d := openTestDB(t)

	require.NoError(t, d.WriteEvent(db.Event{
		EnvName:     "dev-alice",
		ActorEmail:  "alice@example.com",
		ActionType:  "create",
		Description: "Created",
		Outcome:     db.OutcomeInProgress,
	}))

	require.NoError(t, d.UpdateOutcome("dev-alice", "create", db.OutcomeSuccess))

	events, _ := d.ListEvents("dev-alice", 10, 0)
	assert.Equal(t, db.OutcomeSuccess, events[0].Outcome)
}
