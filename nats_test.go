package via

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHandler struct {
	id     int64
	fn     func([]byte)
	active atomic.Bool
}

// mockPubSub implements PubSub for testing without NATS.
type mockPubSub struct {
	mu    sync.Mutex
	subs  map[string][]*mockHandler
	nextID atomic.Int64
}

func newMockPubSub() *mockPubSub {
	return &mockPubSub{subs: make(map[string][]*mockHandler)}
}

func (m *mockPubSub) Publish(subject string, data []byte) error {
	m.mu.Lock()
	handlers := make([]*mockHandler, len(m.subs[subject]))
	copy(handlers, m.subs[subject])
	m.mu.Unlock()
	for _, h := range handlers {
		if h.active.Load() {
			h.fn(data)
		}
	}
	return nil
}

func (m *mockPubSub) Subscribe(subject string, handler func(data []byte)) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mh := &mockHandler{
		id: m.nextID.Add(1),
		fn: handler,
	}
	mh.active.Store(true)
	m.subs[subject] = append(m.subs[subject], mh)
	return &mockSub{handler: mh}, nil
}

func (m *mockPubSub) Close() error { return nil }

type mockSub struct {
	handler *mockHandler
}

func (s *mockSub) Unsubscribe() error {
	s.handler.active.Store(false)
	return nil
}

func TestPubSub_RoundTrip(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	var received []byte
	var wg sync.WaitGroup
	wg.Add(1)

	c := newContext("test-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	_, err := c.Subscribe("test.topic", func(data []byte) {
		received = data
		wg.Done()
	})
	require.NoError(t, err)

	err = c.Publish("test.topic", []byte("hello"))
	require.NoError(t, err)

	wg.Wait()
	assert.Equal(t, []byte("hello"), received)
}

func TestPubSub_MultipleSubscribers(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

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
	wg.Wait()

	assert.Len(t, results, 2)
	assert.Contains(t, results, "c1:msg")
	assert.Contains(t, results, "c2:msg")
}

func TestPubSub_SubscriptionCleanupOnDispose(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	c := newContext("cleanup-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	c.Subscribe("room.1", func(data []byte) {})
	c.Subscribe("room.2", func(data []byte) {})

	assert.Len(t, c.subscriptions, 2)

	c.unsubscribeAll()
	assert.Empty(t, c.subscriptions)
}

func TestPubSub_ManualUnsubscribe(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	c := newContext("unsub-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	called := false
	sub, err := c.Subscribe("topic", func(data []byte) {
		called = true
	})
	require.NoError(t, err)

	sub.Unsubscribe()

	c.Publish("topic", []byte("ignored"))
	time.Sleep(10 * time.Millisecond)
	assert.False(t, called)
}

func TestPubSub_NoOpWhenNotConfigured(t *testing.T) {
	v := New()

	c := newContext("noop-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	err := c.Publish("topic", []byte("data"))
	assert.Error(t, err)

	sub, err := c.Subscribe("topic", func(data []byte) {})
	assert.Error(t, err)
	assert.Nil(t, sub)
}

func TestPubSub_NoOpDuringPanicCheck(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	// Panic-check context has id=""
	c := newContext("", "/", v)

	err := c.Publish("topic", []byte("data"))
	assert.NoError(t, err)

	sub, err := c.Subscribe("topic", func(data []byte) {})
	assert.NoError(t, err)
	assert.Nil(t, sub)
}
