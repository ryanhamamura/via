package via

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/delaneyj/toolbelt/embeddednats"
	"github.com/nats-io/nats.go"
)

// defaultNATS is the process-scoped embedded NATS server.
type defaultNATS struct {
	server  *embeddednats.Server
	nc      *nats.Conn
	js      nats.JetStreamContext
	cancel  context.CancelFunc
	dataDir string
}

var (
	sharedNATS     *defaultNATS
	sharedNATSOnce sync.Once
	sharedNATSErr  error
)

// getSharedNATS returns a process-level singleton embedded NATS server.
// The server starts once and is reused across all V instances.
func getSharedNATS() (*defaultNATS, error) {
	sharedNATSOnce.Do(func() {
		sharedNATS, sharedNATSErr = startDefaultNATS()
	})
	return sharedNATS, sharedNATSErr
}

func startDefaultNATS() (dn *defaultNATS, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("nats server panic: %v", r)
		}
	}()

	dataDir, err := os.MkdirTemp("", "via-nats-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ns, err := embeddednats.New(ctx, embeddednats.WithDirectory(dataDir))
	if err != nil {
		cancel()
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("start embedded nats: %w", err)
	}
	ready := make(chan struct{})
	go func() {
		ns.WaitForServer()
		close(ready)
	}()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		ns.Close()
		cancel()
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("embedded nats server did not start within 10s")
	}

	nc, err := ns.Client()
	if err != nil {
		ns.Close()
		cancel()
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("connect nats client: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		ns.Close()
		cancel()
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("init jetstream: %w", err)
	}

	return &defaultNATS{
		server:  ns,
		nc:      nc,
		js:      js,
		cancel:  cancel,
		dataDir: dataDir,
	}, nil
}

func (n *defaultNATS) Publish(subject string, data []byte) error {
	return n.nc.Publish(subject, data)
}

func (n *defaultNATS) Subscribe(subject string, handler func(data []byte)) (Subscription, error) {
	sub, err := n.nc.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// natsRef wraps a shared defaultNATS as a PubSub. Close is a no-op because
// the underlying server is process-scoped and outlives individual V instances.
type natsRef struct {
	dn *defaultNATS
}

func (r *natsRef) Publish(subject string, data []byte) error {
	return r.dn.Publish(subject, data)
}

func (r *natsRef) Subscribe(subject string, handler func(data []byte)) (Subscription, error) {
	return r.dn.Subscribe(subject, handler)
}

func (r *natsRef) Close() error {
	return nil
}

// NATSConn returns the underlying NATS connection from the built-in embedded
// server, or nil if a custom PubSub backend is in use.
func (v *V) NATSConn() *nats.Conn {
	if v.defaultNATS != nil {
		return v.defaultNATS.nc
	}
	return nil
}

// JetStream returns the JetStream context from the built-in embedded server,
// or nil if a custom PubSub backend is in use.
func (v *V) JetStream() nats.JetStreamContext {
	if v.defaultNATS != nil {
		return v.defaultNATS.js
	}
	return nil
}

// StreamConfig holds the parameters for creating or updating a JetStream stream.
type StreamConfig struct {
	Name     string
	Subjects []string
	MaxMsgs  int64
	MaxAge   time.Duration
}

// EnsureStream creates or updates a JetStream stream matching cfg.
func EnsureStream(v *V, cfg StreamConfig) error {
	js := v.JetStream()
	if js == nil {
		return fmt.Errorf("jetstream not available")
	}
	_, err := js.AddStream(&nats.StreamConfig{
		Name:      cfg.Name,
		Subjects:  cfg.Subjects,
		Retention: nats.LimitsPolicy,
		MaxMsgs:   cfg.MaxMsgs,
		MaxAge:    cfg.MaxAge,
	})
	return err
}

// ReplayHistory fetches the last limit messages from subject,
// deserializing each as T. Returns an empty slice if nothing is available.
func ReplayHistory[T any](v *V, subject string, limit int) ([]T, error) {
	js := v.JetStream()
	if js == nil {
		return nil, fmt.Errorf("jetstream not available")
	}
	sub, err := js.SubscribeSync(subject, nats.DeliverAll(), nats.OrderedConsumer())
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()

	var msgs []T
	for {
		raw, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		var msg T
		if json.Unmarshal(raw.Data, &msg) == nil {
			msgs = append(msgs, msg)
		}
	}

	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}
