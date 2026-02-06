package via

import (
	"sync"
	"testing"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishSubscribe_RoundTrip(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	type event struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var got event
	var wg sync.WaitGroup
	wg.Add(1)

	c := newContext("typed-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	_, err := Subscribe(c, "events", func(e event) {
		got = e
		wg.Done()
	})
	require.NoError(t, err)

	err = Publish(c, "events", event{Name: "click", Count: 42})
	require.NoError(t, err)

	wg.Wait()
	assert.Equal(t, "click", got.Name)
	assert.Equal(t, 42, got.Count)
}

func TestSubscribe_SkipsBadJSON(t *testing.T) {
	ps := newMockPubSub()
	v := New()
	v.Config(Options{PubSub: ps})

	type msg struct {
		Text string `json:"text"`
	}

	called := false
	c := newContext("bad-json-ctx", "/", v)
	c.View(func() h.H { return h.Div() })

	_, err := Subscribe(c, "topic", func(m msg) {
		called = true
	})
	require.NoError(t, err)

	// Publish raw invalid JSON — handler should silently skip
	err = c.Publish("topic", []byte("not json"))
	require.NoError(t, err)

	assert.False(t, called)
}
