package via

// PubSub is an interface for publish/subscribe messaging backends.
// By default, New() starts an embedded NATS server. Supply a custom
// implementation via Config(Options{PubSub: yourBackend}) to override.
type PubSub interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, handler func(data []byte)) (Subscription, error)
	Close() error
}

// Subscription represents an active subscription that can be manually unsubscribed.
type Subscription interface {
	Unsubscribe() error
}
