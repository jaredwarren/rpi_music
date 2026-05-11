package rfid

import (
	"context"
	"testing"
	"time"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/stretchr/testify/assert"
)

func TestEmitEventSendsUID(t *testing.T) {
	events := make(chan Event, 1)
	r := &Reader{
		events: events,
		logger: log.NewNoOpLogger(),
	}

	ok := r.emitEvent(context.Background(), "AABBCCDD")
	assert.True(t, ok)

	select {
	case ev := <-events:
		assert.Equal(t, "AABBCCDD", ev.UID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected RFID event on channel")
	}
}

func TestEmitEventReturnsFalseWhenContextDone(t *testing.T) {
	events := make(chan Event)
	r := &Reader{
		events: events,
		logger: log.NewNoOpLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok := r.emitEvent(ctx, "AABBCCDD")
	assert.False(t, ok)
}
