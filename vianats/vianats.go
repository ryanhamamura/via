// Package vianats provides an embedded NATS server with JetStream as a
// pub/sub backend for Via applications.
package vianats

import (
	"context"
	"fmt"

	"github.com/delaneyj/toolbelt/embeddednats"
	"github.com/nats-io/nats.go"
	"github.com/ryanhamamura/via"
)

// NATS implements via.PubSub using an embedded NATS server with JetStream.
type NATS struct {
	server *embeddednats.Server
	nc     *nats.Conn
	js     nats.JetStreamContext
}

// New starts an embedded NATS server with JetStream enabled and returns a
// ready-to-use NATS instance. The server stores data in dataDir and shuts
// down when ctx is cancelled.
func New(ctx context.Context, dataDir string) (*NATS, error) {
	ns, err := embeddednats.New(ctx, embeddednats.WithDirectory(dataDir))
	if err != nil {
		return nil, fmt.Errorf("vianats: start server: %w", err)
	}
	ns.WaitForServer()

	nc, err := ns.Client()
	if err != nil {
		ns.Close()
		return nil, fmt.Errorf("vianats: connect client: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		ns.Close()
		return nil, fmt.Errorf("vianats: init jetstream: %w", err)
	}

	return &NATS{server: ns, nc: nc, js: js}, nil
}

// Publish sends data to the given subject using core NATS publish.
// JetStream captures messages automatically if a matching stream exists.
func (n *NATS) Publish(subject string, data []byte) error {
	return n.nc.Publish(subject, data)
}

// Subscribe creates a core NATS subscription for real-time fan-out delivery.
func (n *NATS) Subscribe(subject string, handler func(data []byte)) (via.Subscription, error) {
	sub, err := n.nc.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Close shuts down the client connection and embedded server.
func (n *NATS) Close() error {
	n.nc.Close()
	return n.server.Close()
}

// Conn returns the underlying NATS connection for advanced usage.
func (n *NATS) Conn() *nats.Conn {
	return n.nc
}

// JetStream returns the JetStream context for stream configuration and replay.
func (n *NATS) JetStream() nats.JetStreamContext {
	return n.js
}
