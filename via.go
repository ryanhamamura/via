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
	_ "embed"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	sessionManager       *scs.SessionManager
	pubsub               PubSub
	actionRateLimit      RateLimitConfig
	datastarPath         string
	datastarContent      []byte
	datastarOnce         sync.Once
	reaperStop           chan struct{}
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
		v.pubsub = cfg.PubSub
	}
	if cfg.ContextTTL != 0 {
		v.cfg.ContextTTL = cfg.ContextTTL
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
	v.ensureDatastarHandler()
	// check for panics
	func() {
		defer func() {
			if err := recover(); err != nil {
				v.logFatal("failed to register page with init func that panics: %v", err)
				panic(err)
			}
		}()
		c := newContext("", "", v)
		initContextFn(c)
		c.view()
		c.stopAllRoutines()
	}()

	// save page init function allows devmode to restore persisted ctx later
	if v.cfg.DevMode {
		v.devModePageInitFnMap[route] = initContextFn
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
		initContextFn(c)
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
			HTMLAttrs: []h.H{},
		})
		_ = view.Render(w)
	}))
}

func (v *V) registerCtx(c *Context) {
	v.contextRegistryMutex.Lock()
	defer v.contextRegistryMutex.Unlock()
	if c == nil {
		v.logErr(c, "failed to add nil context to registry")
		return
	}
	v.contextRegistry[c.id] = c
	v.logDebug(c, "new context added to registry")
	v.logDebug(nil, "number of sessions in registry: %d", v.currSessionNum())
}

func (v *V) currSessionNum() int {
	return len(v.contextRegistry)
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
	v.logDebug(nil, "number of sessions in registry: %d", v.currSessionNum())
}

func (v *V) getCtx(id string) (*Context, error) {
	v.contextRegistryMutex.RLock()
	defer v.contextRegistryMutex.RUnlock()
	if c, ok := v.contextRegistry[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("ctx '%s' not found", id)
}

func (v *V) startReaper() {
	ttl := v.cfg.ContextTTL
	if ttl < 0 {
		return
	}
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	interval := ttl / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	v.reaperStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-v.reaperStop:
				return
			case <-ticker.C:
				v.reapOrphanedContexts(ttl)
			}
		}
	}()
}

func (v *V) reapOrphanedContexts(ttl time.Duration) {
	now := time.Now()
	v.contextRegistryMutex.RLock()
	var orphans []*Context
	for _, c := range v.contextRegistry {
		if !c.sseConnected.Load() && now.Sub(c.createdAt) > ttl {
			orphans = append(orphans, c)
		}
	}
	v.contextRegistryMutex.RUnlock()

	for _, c := range orphans {
		v.logInfo(c, "reaping orphaned context (no SSE connection after %s)", ttl)
		v.cleanupCtx(c)
	}
}

// Start starts the Via HTTP server and blocks until a SIGINT or SIGTERM
// signal is received, then performs a graceful shutdown.
func (v *V) Start() {
	handler := http.Handler(v.mux)
	if v.sessionManager != nil {
		handler = v.sessionManager.LoadAndSave(v.mux)
	}
	v.server = &http.Server{
		Addr:    v.cfg.ServerAddress,
		Handler: handler,
	}

	v.startReaper()

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

	v.shutdown()
}

// Shutdown gracefully shuts down the server and all contexts.
// Safe for programmatic or test use.
func (v *V) Shutdown() {
	v.shutdown()
}

func (v *V) shutdown() {
	if v.reaperStop != nil {
		close(v.reaperStop)
	}
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
			v.logErr(nil, "sse stream failed to start: %v", err)
			return
		}
		c.reqCtx = r.Context()

		sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli(datastar.WithBrotliLevel(5))))

		// use last-event-id to tell if request is a sse reconnect
		sse.Send(datastar.EventTypePatchElements, []string{}, datastar.WithSSEEventId("via"))

		c.sseConnected.Store(true)
		v.logDebug(c, "SSE connection established")

		go func() {
			c.Sync()
		}()

		for {
			select {
			case <-sse.Context().Done():
				v.logDebug(c, "SSE connection ended")
				v.cleanupCtx(c)
				return
			case <-c.ctxDisposedChan:
				v.logDebug(c, "context disposed, closing SSE")
				return
			case patch := <-c.patchChan:
				switch patch.typ {
				case patchTypeElements:
					if err := sse.PatchElements(patch.content); err != nil {
						// Only log if connection wasn't closed (avoids noise during shutdown/tests)
						if sse.Context().Err() == nil {
							v.logErr(c, "PatchElements failed: %v", err)
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
		entry.fn()
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
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:8]
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
			continue
		}
	}
	return params
}
