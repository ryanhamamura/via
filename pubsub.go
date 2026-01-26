package via

// PubSub is an interface for publish/subscribe messaging backends.
// The vianats sub-package provides an embedded NATS implementation.
type PubSub interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, handler func(data []byte)) (Subscription, error)
	Close() error
}

// Subscription represents an active subscription that can be manually unsubscribed.
type Subscription interface {
	Unsubscribe() error
}
