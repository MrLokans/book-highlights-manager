package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObsidianSyncScheduler_StateManagement(t *testing.T) {
	t.Run("initially not running", func(t *testing.T) {
		sched := NewObsidianSyncScheduler(nil, nil, nil, nil)
		assert.False(t, sched.IsRunning())
		assert.Nil(t, sched.GetNextRunTime())
	})

	t.Run("stop is no-op when not running", func(t *testing.T) {
		sched := NewObsidianSyncScheduler(nil, nil, nil, nil)
		sched.Stop() // should not panic
		assert.False(t, sched.IsRunning())
	})
}
