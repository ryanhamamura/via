package via

import (
	"sync"
	"testing"
	"time"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubSub_RoundTrip(t *testing.T) {
	v := New()
	defer v.Shutdown()

	var received []byte
	done := make(chan struct{})

	c := newContext("test-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	_, err := c.Subscribe("test.topic", func(data []byte) {
		received = data
		close(done)
	})
	require.NoError(t, err)

	err = c.Publish("test.topic", []byte("hello"))
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
	assert.Equal(t, []byte("hello"), received)
}

func TestPubSub_MultipleSubscribers(t *testing.T) {
	v := New()
	defer v.Shutdown()

	var mu sync.Mutex
	var results []string
	var wg sync.WaitGroup
	wg.Add(2)

	c1 := newContext("ctx-1", "/", v)
	c1.View(func() h.H { return h.Div() })
	c2 := newContext("ctx-2", "/", v)
	c2.View(func() h.H { return h.Div() })

	c1.Subscribe("broadcast", func(data []byte) {
		mu.Lock()
		results = append(results, "c1:"+string(data))
		mu.Unlock()
		wg.Done()
	})

	c2.Subscribe("broadcast", func(data []byte) {
		mu.Lock()
		results = append(results, "c2:"+string(data))
		mu.Unlock()
		wg.Done()
	})

	c1.Publish("broadcast", []byte("msg"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for messages")
	}

	assert.Len(t, results, 2)
	assert.Contains(t, results, "c1:msg")
	assert.Contains(t, results, "c2:msg")
}

func TestPubSub_SubscriptionCleanupOnDispose(t *testing.T) {
	v := New()
	defer v.Shutdown()

	c := newContext("cleanup-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	c.Subscribe("room.1", func(data []byte) {})
	c.Subscribe("room.2", func(data []byte) {})

	assert.Len(t, c.subscriptions, 2)

	c.unsubscribeAll()
	assert.Empty(t, c.subscriptions)
}

func TestPubSub_ManualUnsubscribe(t *testing.T) {
	v := New()
	defer v.Shutdown()

	c := newContext("unsub-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	called := false
	sub, err := c.Subscribe("topic", func(data []byte) {
		called = true
	})
	require.NoError(t, err)

	sub.Unsubscribe()

	c.Publish("topic", []byte("ignored"))
	time.Sleep(50 * time.Millisecond)
	assert.False(t, called)
}

func TestPubSub_NoOpDuringPanicCheck(t *testing.T) {
	v := New()
	defer v.Shutdown()

	// Panic-check context has id=""
	c := newContext("", "/", v)

	err := c.Publish("topic", []byte("data"))
	assert.NoError(t, err)

	sub, err := c.Subscribe("topic", func(data []byte) {})
	assert.NoError(t, err)
	assert.Nil(t, sub)
}
