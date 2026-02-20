// Package via provides a reactive, real-time engine for creating Go web
// applications. It lets you build live, type-safe web interfaces without
// JavaScript.
//
// Via unifies routing, state, and UI reactivity through a simple mental model:
// Go on the server — HTML in the browser — updated in real time via Datastar.
package via

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	ossignal "os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/rs/zerolog"
	"github.com/ryanhamamura/via/h"
	"github.com/starfederation/datastar-go/datastar"
)

//go:embed datastar.js
var datastarJS []byte

//go:embed navigate.js
var navigateJS []byte

// V is the root application.
// It manages page routing, user sessions, and SSE connections for live updates.
type V struct {
	cfg                  Options
	mux                  *http.ServeMux
	server               *http.Server
	logger               zerolog.Logger
	contextRegistry      map[string]*Context
	contextRegistryMutex sync.RWMutex
	documentHeadIncludes []h.H
	documentFootIncludes []h.H
	devModePageInitFnMap map[string]func(*Context)
	pageRegistry         map[string]func(*Context)
	sessionManager       *scs.SessionManager
	pubsub               PubSub
	defaultNATS          *defaultNATS
	actionRateLimit      RateLimitConfig
	datastarPath         string
	datastarContent      []byte
	datastarOnce         sync.Once
	middleware           []Middleware
	layout               func(func() h.H) h.H
}

func (v *V) logEvent(evt *zerolog.Event, c *Context) *zerolog.Event {
	if c != nil && c.id != "" {
		evt = evt.Str("via-ctx", c.id)
	}
	return evt
}

func (v *V) logFatal(format string, a ...any) {
	v.logEvent(v.logger.WithLevel(zerolog.FatalLevel), nil).Msgf(format, a...)
}

func (v *V) logErr(c *Context, format string, a ...any) {
	v.logEvent(v.logger.Error(), c).Msgf(format, a...)
}

func (v *V) logWarn(c *Context, format string, a ...any) {
	v.logEvent(v.logger.Warn(), c).Msgf(format, a...)
}

func (v *V) logInfo(c *Context, format string, a ...any) {
	v.logEvent(v.logger.Info(), c).Msgf(format, a...)
}

func (v *V) logDebug(c *Context, format string, a ...any) {
	v.logEvent(v.logger.Debug(), c).Msgf(format, a...)
}

func newConsoleLogger(level zerolog.Level) zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		With().Timestamp().Logger().Level(level)
}

// Config overrides the default configuration with the given options.
func (v *V) Config(cfg Options) {
	if cfg.Logger != nil {
		v.logger = *cfg.Logger
	} else if cfg.LogLevel != nil || cfg.DevMode != v.cfg.DevMode {
		level := zerolog.InfoLevel
		if cfg.LogLevel != nil {
			level = *cfg.LogLevel
		}
		if cfg.DevMode {
			v.logger = newConsoleLogger(level)
		} else {
			v.logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(level)
		}
	}
	if cfg.DocumentTitle != "" {
		v.cfg.DocumentTitle = cfg.DocumentTitle
	}
	if cfg.Plugins != nil {
		for _, plugin := range cfg.Plugins {
			if plugin != nil {
				plugin(v)
			}
		}
	}
	if cfg.DevMode != v.cfg.DevMode {
		v.cfg.DevMode = cfg.DevMode
	}
	if cfg.ServerAddress != "" {
		v.cfg.ServerAddress = cfg.ServerAddress
	}
	if cfg.SessionManager != nil {
		v.sessionManager = cfg.SessionManager
	}
	if cfg.DatastarContent != nil {
		v.datastarContent = cfg.DatastarContent
	}
	if cfg.DatastarPath != "" {
		v.datastarPath = cfg.DatastarPath
	}
	if cfg.PubSub != nil {
		v.defaultNATS = nil
		v.pubsub = cfg.PubSub
	}
	if cfg.Streams != nil {
		v.cfg.Streams = cfg.Streams
	}
	if cfg.ActionRateLimit.Rate != 0 || cfg.ActionRateLimit.Burst != 0 {
		v.actionRateLimit = cfg.ActionRateLimit
	}
}

// AppendToHead appends the given h.H nodes to the head of the base HTML document.
// Useful for including css stylesheets and JS scripts.
func (v *V) AppendToHead(elements ...h.H) {
	for _, el := range elements {
		if el != nil {
			v.documentHeadIncludes = append(v.documentHeadIncludes, el)
		}
	}
}

// AppendToFoot appends the given h.H nodes to the end of the base HTML document body.
// Useful for including JS scripts.
func (v *V) AppendToFoot(elements ...h.H) {
	for _, el := range elements {
		if el != nil {
			v.documentFootIncludes = append(v.documentFootIncludes, el)
		}
	}
}

// Page registers a route and its associated page handler. The handler receives a *Context
// that defines state, UI, signals, and actions.
//
// Example:
//
//	v.Page("/", func(c *via.Context) {
//		c.View(func() h.H {
//			return h.H1(h.Text("Hello, Via!"))
//		})
//	})
func (v *V) Page(route string, initContextFn func(c *Context)) {
	wrapped := chainMiddleware(v.middleware, initContextFn)
	v.page(route, initContextFn, wrapped)
}

// page registers a route with separate raw and wrapped init functions.
// raw is used for the panic-check at registration time; wrapped includes
// any middleware and is used as the live handler.
func (v *V) page(route string, raw, wrapped func(*Context)) {
	v.ensureDatastarHandler()
	// check for panics using the raw handler (no middleware)
	func() {
		defer func() {
			if err := recover(); err != nil {
				v.logFatal("failed to register page with init func that panics: %v", err)
				panic(err)
			}
		}()
		c := newContext("", "", v)
		raw(c)
		c.view()
		c.stopAllRoutines()
	}()

	v.pageRegistry[route] = wrapped
	if v.cfg.DevMode {
		v.devModePageInitFnMap[route] = wrapped
	}
	v.mux.HandleFunc("GET "+route, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v.logDebug(nil, "GET %s", r.URL.String())
		if strings.Contains(r.URL.Path, "favicon") ||
			strings.Contains(r.URL.Path, ".well-known") ||
			strings.Contains(r.URL.Path, "js.map") {
			return
		}
		id := fmt.Sprintf("%s_/%s", route, genRandID())
		c := newContext(id, route, v)
		c.reqCtx = r.Context()
		routeParams := extractParams(route, r.URL.Path)
		c.injectRouteParams(routeParams)
		wrapped(c)
		v.registerCtx(c)
		if v.cfg.DevMode {
			v.devModePersist(c)
		}
		headElements := []h.H{h.Script(h.Type("module"), h.Src(v.datastarPath))}
		headElements = append(headElements, v.documentHeadIncludes...)
		headElements = append(headElements,
			h.Meta(h.Data("signals", fmt.Sprintf("{'via-ctx':'%s','via-csrf':'%s'}", id, c.csrfToken))),
			h.Meta(h.Data("init", "@get('/_sse')")),
			h.Meta(h.Data("init", fmt.Sprintf(`window.addEventListener('beforeunload', (evt) => {
			navigator.sendBeacon('/_session/close', '%s');});`, c.id))),
			h.Meta(h.Attr("name", "view-transition"), h.Attr("content", "same-origin")),
			h.Script(h.Raw(string(navigateJS))),
		)

		bodyElements := []h.H{c.view()}
		bodyElements = append(bodyElements, v.documentFootIncludes...)
		if v.cfg.DevMode {
			bodyElements = append(bodyElements, h.Script(h.Type("module"),
				h.Src("https://cdn.jsdelivr.net/gh/dataSPA/dataSPA-inspector@latest/dataspa-inspector.bundled.js")))
			bodyElements = append(bodyElements, h.Raw("<dataspa-inspector/>"))
		}
		view := h.HTML5(h.HTML5Props{
			Title:     v.cfg.DocumentTitle,
			Head:      headElements,
			Body:      bodyElements,
			})
		_ = view.Render(w)
	}))
}

func (v *V) registerCtx(c *Context) {
	v.contextRegistryMutex.Lock()
	defer v.contextRegistryMutex.Unlock()
	v.contextRegistry[c.id] = c
	v.logDebug(c, "new context added to registry")
	v.logDebug(nil, "number of sessions in registry: %d", len(v.contextRegistry))
}

func (v *V) cleanupCtx(c *Context) {
	c.dispose()
	if v.cfg.DevMode {
		v.devModeRemovePersisted(c)
	}
	v.unregisterCtx(c)
}

func (v *V) unregisterCtx(c *Context) {
	if c.id == "" {
		v.logErr(c, "unregister ctx failed: ctx contains empty id")
		return
	}
	v.contextRegistryMutex.Lock()
	defer v.contextRegistryMutex.Unlock()
	v.logDebug(c, "ctx removed from registry")
	delete(v.contextRegistry, c.id)
	v.logDebug(nil, "number of sessions in registry: %d", len(v.contextRegistry))
}

func (v *V) getCtx(id string) (*Context, error) {
	v.contextRegistryMutex.RLock()
	defer v.contextRegistryMutex.RUnlock()
	if c, ok := v.contextRegistry[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("ctx '%s' not found", id)
}

// Start starts the Via HTTP server and blocks until a SIGINT or SIGTERM
// signal is received, then performs a graceful shutdown.
func (v *V) Start() {
	if v.pubsub == nil {
		dn, err := getSharedNATS()
		if err != nil {
			v.logWarn(nil, "embedded NATS unavailable: %v", err)
		} else {
			v.defaultNATS = dn
			v.pubsub = &natsRef{dn: dn}
		}
	}

	for _, sc := range v.cfg.Streams {
		if err := EnsureStream(v, sc); err != nil {
			v.logger.Fatal().Err(err).Msgf("failed to create stream %q", sc.Name)
		}
	}

	handler := http.Handler(v.mux)
	if v.sessionManager != nil {
		handler = v.sessionManager.LoadAndSave(v.mux)
	}
	v.server = &http.Server{
		Addr:    v.cfg.ServerAddress,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- v.server.ListenAndServe()
	}()

	v.logInfo(nil, "via started at [%s]", v.cfg.ServerAddress)

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		v.logInfo(nil, "received signal %v, shutting down", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			v.logger.Fatal().Err(err).Msg("http server failed")
		}
		return
	}

	v.Shutdown()
}

// Shutdown gracefully shuts down the server and all contexts.
// Safe for programmatic or test use.
func (v *V) Shutdown() {
	v.logInfo(nil, "draining all contexts")
	v.drainAllContexts()

	if v.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := v.server.Shutdown(ctx); err != nil {
			v.logErr(nil, "http server shutdown error: %v", err)
		}
	}

	if v.pubsub != nil {
		if err := v.pubsub.Close(); err != nil {
			v.logErr(nil, "pubsub close error: %v", err)
		}
	}
	v.defaultNATS = nil

	v.logInfo(nil, "shutdown complete")
}

func (v *V) drainAllContexts() {
	v.contextRegistryMutex.Lock()
	contexts := make([]*Context, 0, len(v.contextRegistry))
	for _, c := range v.contextRegistry {
		contexts = append(contexts, c)
	}
	v.contextRegistry = make(map[string]*Context)
	v.contextRegistryMutex.Unlock()

	for _, c := range contexts {
		v.logDebug(c, "disposing context")
		c.dispose()
	}
	v.logInfo(nil, "drained %d context(s)", len(contexts))
}

// HTTPServeMux returns the underlying HTTP request multiplexer to enable user extentions, middleware and
// plugins. It also enables integration with test frameworks like gost-dom/browser for SSE/Datastar testing.
//
// IMPORTANT. The returned *http.ServeMux can only be modified during initialization, before calling via.Start().
// Concurrent handler registration is not safe.
func (v *V) HTTPServeMux() *http.ServeMux {
	return v.mux
}

// PubSub returns the configured PubSub backend, or nil if none is set.
func (v *V) PubSub() PubSub {
	return v.pubsub
}

// Static serves files from a filesystem directory at the given URL prefix.
//
// Example:
//
//	v.Static("/assets/", "./public")
func (v *V) Static(urlPrefix, dir string) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	fileServer := http.StripPrefix(urlPrefix, http.FileServer(http.Dir(dir)))
	v.mux.Handle("GET "+urlPrefix, noDirListing(fileServer))
}

// StaticFS serves files from an [fs.FS] at the given URL prefix.
// This is useful with //go:embed filesystems.
//
// Example:
//
//	//go:embed static
//	var staticFiles embed.FS
//	v.StaticFS("/assets/", staticFiles)
func (v *V) StaticFS(urlPrefix string, fsys fs.FS) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	fileServer := http.StripPrefix(urlPrefix, http.FileServerFS(fsys))
	v.mux.Handle("GET "+urlPrefix, noDirListing(fileServer))
}

// noDirListing wraps a file server handler to return 404 for directory requests.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (v *V) ensureDatastarHandler() {
	v.datastarOnce.Do(func() {
		v.mux.HandleFunc("GET "+v.datastarPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write(v.datastarContent)
		})
	})
}

func (v *V) devModePersist(c *Context) {
	p := filepath.Join(".via", "devmode", "ctx.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		v.logFatal("failed to create directory for devmode files: %v", err)
	}

	// load persisted list from file, or empty list if file not found
	file, err := os.Open(p)
	ctxRegMap := make(map[string]string)
	if err == nil {
		json.NewDecoder(file).Decode(&ctxRegMap)
	}
	file.Close()

	// add ctx to persisted list
	if _, ok := ctxRegMap[c.id]; !ok {
		ctxRegMap[c.id] = c.route
	}

	// write persisted list to file
	file, err = os.Create(p)
	if err != nil {
		v.logErr(c, "devmode failed to percist ctx: %v", err)

	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(ctxRegMap); err != nil {
		v.logErr(c, "devmode failed to persist ctx")
	}
	v.logDebug(c, "devmode persisted ctx to file")
}

func (v *V) devModeRemovePersisted(c *Context) {
	p := filepath.Join(".via", "devmode", "ctx.json")

	// load persisted list from file, or empty list if file not found
	file, err := os.Open(p)
	ctxRegMap := make(map[string]string)
	if err == nil {
		json.NewDecoder(file).Decode(&ctxRegMap)
	}
	file.Close()

	delete(ctxRegMap, c.id)

	// write persisted list to file
	file, err = os.Create(p)
	if err != nil {
		v.logErr(c, "devmode failed to remove percisted ctx: %v", err)

	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(ctxRegMap); err != nil {
		v.logErr(c, "devmode failed to remove persisted ctx")
	}
	v.logDebug(c, "devmode removed persisted ctx from file")
}

func (v *V) devModeRestore(cID string) {
	p := filepath.Join(".via", "devmode", "ctx.json")
	file, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		v.logErr(nil, "devmode could not restore ctx from file: %v", err)
		return
	}
	defer file.Close()
	var ctxRegMap map[string]string
	if err := json.NewDecoder(file).Decode(&ctxRegMap); err != nil {
		v.logWarn(nil, "devmode could not restore ctx from file: %v", err)
		return
	}
	for ctxID, pageRoute := range ctxRegMap {
		if ctxID == cID {
			pageInitFn, ok := v.devModePageInitFnMap[pageRoute]
			if !ok {
				v.logWarn(nil, "devmode could not restore ctx from file: page init fn for route '%s' not found", pageRoute)
				continue
			}
			c := newContext(ctxID, pageRoute, v)
			pageInitFn(c)
			v.registerCtx(c)
			v.logDebug(c, "devmode restored ctx")
		}
	}
}

type patchType int

const (
	patchTypeElements = iota
	patchTypeElementsWithVT
	patchTypeSignals
	patchTypeScript
	patchTypeRedirect
	patchTypeReplaceURL
)

type patch struct {
	typ     patchType
	content string
}

// New creates a new *V application with default configuration.
func New() *V {
	mux := http.NewServeMux()

	v := &V{
		mux:                  mux,
		logger:               newConsoleLogger(zerolog.InfoLevel),
		contextRegistry:      make(map[string]*Context),
		devModePageInitFnMap: make(map[string]func(*Context)),
		pageRegistry:         make(map[string]func(*Context)),
		sessionManager:       scs.New(),
		datastarPath:         "/_datastar.js",
		datastarContent:      datastarJS,
		cfg: Options{
			DevMode:       false,
			ServerAddress: ":3000",
			DocumentTitle: "⚡ Via",
		},
	}

	v.mux.HandleFunc("GET /_sse", func(w http.ResponseWriter, r *http.Request) {
		var sigs map[string]any
		_ = datastar.ReadSignals(r, &sigs)
		cID, _ := sigs["via-ctx"].(string)

		if v.cfg.DevMode {
			if _, err := v.getCtx(cID); err != nil {
				v.devModeRestore(cID)
			}
		}
		c, err := v.getCtx(cID)
		if err != nil {
			v.logInfo(nil, "context expired, reloading client: %s", cID)
			sse := datastar.NewSSE(w, r)
			sse.ExecuteScript("window.location.reload()")
			return
		}
		c.reqCtx = r.Context()

		sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli(datastar.WithBrotliLevel(5))))

		// use last-event-id to tell if request is a sse reconnect
		sse.Send(datastar.EventTypePatchElements, []string{}, datastar.WithSSEEventId("via"))

		// Drain stale patches on reconnect so the client gets fresh state
		if c.sseDisconnectedAt.Load() != nil {
			for {
				select {
				case <-c.patchChan:
				default:
					goto drained
				}
			}
		drained:
		}
		c.sseConnected.Store(true)
		c.sseDisconnectedAt.Store(nil)
		v.logDebug(c, "SSE connection established")

		go c.Sync()

		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case <-sse.Context().Done():
				v.logDebug(c, "SSE connection ended")
				c.sseConnected.Store(false)
				dcNow := time.Now()
				c.sseDisconnectedAt.Store(&dcNow)
				return
			case <-c.ctxDisposedChan:
				v.logDebug(c, "context disposed, closing SSE")
				return
			case <-keepalive.C:
				sse.PatchSignals([]byte("{}"))
			case patch := <-c.patchChan:
				switch patch.typ {
				case patchTypeElements:
					if err := sse.PatchElements(patch.content); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "PatchElements failed: %v", err)
						}
					}
				case patchTypeElementsWithVT:
					if err := sse.PatchElements(patch.content, datastar.WithViewTransitions()); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "PatchElements (view transition) failed: %v", err)
						}
					}
				case patchTypeSignals:
					if err := sse.PatchSignals([]byte(patch.content)); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "PatchSignals failed: %v", err)
						}
					}
				case patchTypeScript:
					if err := sse.ExecuteScript(patch.content, datastar.WithExecuteScriptAutoRemove(true)); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "ExecuteScript failed: %v", err)
						}
					}
				case patchTypeRedirect:
					if err := sse.Redirect(patch.content); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "Redirect failed: %v", err)
						}
					}
				case patchTypeReplaceURL:
					parsedURL, err := url.Parse(patch.content)
					if err != nil {
						v.logErr(c, "ReplaceURL failed to parse URL: %v", err)
					} else if err := sse.ReplaceURL(*parsedURL); err != nil {
						if sse.Context().Err() == nil {
							v.logErr(c, "ReplaceURL failed: %v", err)
						}
					}
				}
			}
		}
	})

	v.mux.HandleFunc("GET /_action/{id}", func(w http.ResponseWriter, r *http.Request) {
		actionID := r.PathValue("id")
		var sigs map[string]any
		_ = datastar.ReadSignals(r, &sigs)
		cID, _ := sigs["via-ctx"].(string)
		c, err := v.getCtx(cID)
		if err != nil {
			v.logErr(nil, "action '%s' failed: %v", actionID, err)
			return
		}
		csrfToken, _ := sigs["via-csrf"].(string)
		if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(c.csrfToken)) != 1 {
			v.logWarn(c, "action '%s' rejected: invalid CSRF token", actionID)
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		if c.actionLimiter != nil && !c.actionLimiter.Allow() {
			v.logWarn(c, "action '%s' rate limited", actionID)
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		c.reqCtx = r.Context()
		entry, err := c.getAction(actionID)
		if err != nil {
			v.logDebug(c, "action '%s' failed: %v", actionID, err)
			return
		}
		if entry.limiter != nil && !entry.limiter.Allow() {
			v.logWarn(c, "action '%s' rate limited (per-action)", actionID)
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		// log err if action panics
		defer func() {
			if r := recover(); r != nil {
				v.logErr(c, "action '%s' failed: %v", actionID, r)
			}
		}()

		c.injectSignals(sigs)
		if len(entry.middleware) > 0 {
			chainMiddleware(entry.middleware, func(_ *Context) { entry.fn() })(c)
		} else {
			entry.fn()
		}
	})

	v.mux.HandleFunc("POST /_navigate", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		cID := r.FormValue("via-ctx")
		csrfToken := r.FormValue("via-csrf")
		navURL := r.FormValue("url")
		popstate := r.FormValue("popstate") == "1"

		if cID == "" || navURL == "" || !strings.HasPrefix(navURL, "/") {
			http.Error(w, "missing or invalid parameters", http.StatusBadRequest)
			return
		}
		c, err := v.getCtx(cID)
		if err != nil {
			v.logErr(nil, "navigate failed: %v", err)
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(c.csrfToken)) != 1 {
			v.logWarn(c, "navigate rejected: invalid CSRF token")
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		if c.actionLimiter != nil && !c.actionLimiter.Allow() {
			v.logWarn(c, "navigate rate limited")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		c.reqCtx = r.Context()
		v.logDebug(c, "SPA navigate to %s", navURL)
		c.Navigate(navURL, popstate)
		w.WriteHeader(http.StatusOK)
	})

	v.mux.HandleFunc("POST /_session/close", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			v.logErr(nil, "error reading body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		cID := string(body)
		c, err := v.getCtx(cID)
		if err != nil {
			v.logErr(c, "failed to handle session close: %v", err)
			return
		}
		v.logDebug(c, "session close event triggered")
		v.cleanupCtx(c)
	})

	return v
}

func genRandID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func genCSRFToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func extractParams(pattern, path string) map[string]string {
	p := strings.Split(strings.Trim(pattern, "/"), "/")
	u := strings.Split(strings.Trim(path, "/"), "/")
	if len(p) != len(u) {
		return nil
	}
	params := make(map[string]string)
	for i := range p {
		if strings.HasPrefix(p[i], "{") && strings.HasSuffix(p[i], "}") {
			key := p[i][1 : len(p[i])-1] // remove {}
			params[key] = u[i]
		} else if p[i] != u[i] {
			return nil
		}
	}
	return params
}

// matchRoute finds the registered page init function and extracted params for the given path.
func (v *V) matchRoute(path string) (route string, initFn func(*Context), params map[string]string) {
	for pattern, fn := range v.pageRegistry {
		if p := extractParams(pattern, path); p != nil {
			return pattern, fn, p
		}
	}
	return "", nil, nil
}

// Layout sets a layout function that wraps every page's view.
// The layout receives the page content as a function and returns the full view.
func (v *V) Layout(f func(func() h.H) h.H) {
	v.layout = f
}
