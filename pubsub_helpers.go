package via

import "encoding/json"

// Publish JSON-marshals msg and publishes to subject.
func Publish[T any](c *Context, subject string, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Publish(subject, data)
}

// Subscribe JSON-unmarshals each message as T and calls handler.
func Subscribe[T any](c *Context, subject string, handler func(T)) (Subscription, error) {
	return c.Subscribe(subject, func(data []byte) {
		var msg T
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		handler(msg)
	})
}
