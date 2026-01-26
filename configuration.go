package via

import "github.com/alexedwards/scs/v2"

type LogLevel int

const (
	undefined LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// Plugin is a func that can mutate the given *via.V app runtime. It is useful to integrate popular JS/CSS UI libraries or tools.
type Plugin func(v *V)

// Options defines configuration options for the via application
type Options struct {
	// The development mode flag. If true, enables server and browser auto-reload on `.go` file changes.
	DevMode bool

	// The http server address. e.g. ':3000'
	ServerAddress string

	// Level of the logs to write to stdout.
	// Options: Error, Warn, Info, Debug.
	LogLvl LogLevel

	// The title of the HTML document.
	DocumentTitle string

	// Plugins to extend the capabilities of the `Via` application.
	Plugins []Plugin

	// SessionManager enables cookie-based sessions. If set, Via wraps handlers
	// with scs LoadAndSave middleware. Configure the session manager before
	// passing it (lifetime, cookie settings, store, etc).
	SessionManager *scs.SessionManager

	// DatastarContent is the Datastar.js script content.
	// If nil, the embedded default is used.
	DatastarContent []byte

	// DatastarPath is the URL path where the script is served.
	// Defaults to "/_datastar.js" if empty.
	DatastarPath string

	// PubSub enables publish/subscribe messaging. Use vianats.New() for an
	// embedded NATS backend, or supply any PubSub implementation.
	PubSub PubSub
}
