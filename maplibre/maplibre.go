// Package maplibre provides a Go API for MapLibre GL JS maps within Via applications.
//
// It follows the same ExecScript + DataIgnoreMorph pattern used for other client-side
// JS library integrations (e.g. ECharts in the realtimechart example).
package maplibre

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

//go:embed maplibre-gl.js
var maplibreJS []byte

//go:embed maplibre-gl.css
var maplibreCSS []byte

// Plugin serves the embedded MapLibre GL JS/CSS and injects them into the document head.
func Plugin(v *via.V) {
	v.HTTPServeMux().HandleFunc("GET /_maplibre/maplibre-gl.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(maplibreJS)
	})
	v.HTTPServeMux().HandleFunc("GET /_maplibre/maplibre-gl.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write(maplibreCSS)
	})
	v.AppendToHead(
		h.Link(h.Rel("stylesheet"), h.Href("/_maplibre/maplibre-gl.css")),
		h.Script(h.Src("/_maplibre/maplibre-gl.js")),
	)
}

// Map represents a MapLibre GL map instance bound to a Via context.
type Map struct {
	// Viewport signals — readable with .Text(), .String(), etc.
	CenterLng *via.Signal
	CenterLat *via.Signal
	Zoom      *via.Signal
	Bearing   *via.Signal
	Pitch     *via.Signal

	id   string
	ctx  *via.Context
	opts Options

	sources  []sourceEntry
	layers   []Layer
	markers  []markerEntry
	popups   []popupEntry
	events   []eventEntry
	controls []controlEntry

	rendered bool
}

// New creates a Map bound to the given Via context with the provided options.
// It registers viewport signals on the context for browser → server sync.
func New(c *via.Context, opts Options) *Map {
	if opts.Width == "" {
		opts.Width = "100%"
	}
	if opts.Height == "" {
		opts.Height = "400px"
	}

	m := &Map{
		id:   genID(),
		ctx:  c,
		opts: opts,
	}

	m.CenterLng = c.Signal(opts.Center.Lng)
	m.CenterLat = c.Signal(opts.Center.Lat)
	m.Zoom = c.Signal(opts.Zoom)
	m.Bearing = c.Signal(opts.Bearing)
	m.Pitch = c.Signal(opts.Pitch)

	return m
}

// Element returns the h.H DOM tree for the map. Call this once inside your View function.
// After Element() is called, subsequent source/layer/marker/popup operations
// use ExecScript instead of accumulating for the init script.
//
// Extra children are appended inside the wrapper div (useful for event inputs
// and data-effect binding elements).
func (m *Map) Element(extra ...h.H) h.H {
	m.rendered = true

	children := []h.H{
		h.ID("_vwrap_" + m.id),
		// Map container — morph-ignored so MapLibre's DOM isn't destroyed on Sync()
		h.Div(
			h.ID("_vmap_"+m.id),
			h.DataIgnoreMorph(),
			h.Attr("style", fmt.Sprintf("width:%s;height:%s", m.opts.Width, m.opts.Height)),
		),
		// Hidden inputs for viewport signal binding (outside morph-ignored zone)
		h.Input(h.Type("hidden"), m.CenterLng.Bind()),
		h.Input(h.Type("hidden"), m.CenterLat.Bind()),
		h.Input(h.Type("hidden"), m.Zoom.Bind()),
		h.Input(h.Type("hidden"), m.Bearing.Bind()),
		h.Input(h.Type("hidden"), m.Pitch.Bind()),
	}

	// data-effect elements for signal-backed markers
	for _, me := range m.markers {
		if me.marker.LngSignal != nil && me.marker.LatSignal != nil {
			children = append(children, h.Div(
				h.Attr("style", "display:none"),
				h.DataEffect(markerEffectExpr(m.id, me.id, me.marker)),
			))
		}
	}

	// Hidden inputs for signal-backed marker position/rotation writeback
	for _, me := range m.markers {
		if me.marker.LngSignal != nil && me.marker.LatSignal != nil {
			children = append(children,
				h.Input(h.Type("hidden"), me.marker.LngSignal.Bind()),
				h.Input(h.Type("hidden"), me.marker.LatSignal.Bind()),
			)
			if me.marker.RotationSignal != nil {
				children = append(children,
					h.Input(h.Type("hidden"), me.marker.RotationSignal.Bind()),
				)
			}
		}
		// Hidden inputs for marker event signals
		if me.handle != nil {
			for _, ev := range me.handle.events {
				children = append(children,
					h.Input(h.Type("hidden"), ev.signal.Bind()),
				)
			}
		}
	}

	// Hidden inputs for popup event signals
	for _, pe := range m.popups {
		if pe.handle != nil {
			for _, ev := range pe.handle.events {
				children = append(children,
					h.Input(h.Type("hidden"), ev.signal.Bind()),
				)
			}
		}
	}

	children = append(children, extra...)

	// Init script last
	children = append(children, h.Script(h.Raw(initScript(m))))

	return h.Div(children...)
}

// --- Viewport readers (signal → Go) ---

// Center returns the current map center from synced signals.
func (m *Map) Center() LngLat {
	return LngLat{
		Lng: parseFloat(m.CenterLng.String()),
		Lat: parseFloat(m.CenterLat.String()),
	}
}

// --- Camera methods ---

// FlyTo animates the map to the target camera state.
func (m *Map) FlyTo(opts CameraOptions) {
	m.exec(fmt.Sprintf(`m.flyTo(%s);`, cameraOptionsJS(opts)))
}

// EaseTo eases the map to the target camera state.
func (m *Map) EaseTo(opts CameraOptions) {
	m.exec(fmt.Sprintf(`m.easeTo(%s);`, cameraOptionsJS(opts)))
}

// JumpTo jumps the map to the target camera state without animation.
func (m *Map) JumpTo(opts CameraOptions) {
	m.exec(fmt.Sprintf(`m.jumpTo(%s);`, cameraOptionsJS(opts)))
}

// FitBounds fits the map to the given bounds with optional camera options.
func (m *Map) FitBounds(bounds LngLatBounds, opts ...CameraOptions) {
	boundsJS := fmt.Sprintf("[[%s,%s],[%s,%s]]",
		formatFloat(bounds.SW.Lng), formatFloat(bounds.SW.Lat),
		formatFloat(bounds.NE.Lng), formatFloat(bounds.NE.Lat))
	if len(opts) > 0 {
		m.exec(fmt.Sprintf(`m.fitBounds(%s,%s);`, boundsJS, cameraOptionsJS(opts[0])))
	} else {
		m.exec(fmt.Sprintf(`m.fitBounds(%s);`, boundsJS))
	}
}

// Stop aborts any in-progress camera animation.
func (m *Map) Stop() {
	m.exec(`m.stop();`)
}

// SetCenter sets the map center without animation.
func (m *Map) SetCenter(ll LngLat) {
	m.exec(fmt.Sprintf(`m.setCenter([%s,%s]);`,
		formatFloat(ll.Lng), formatFloat(ll.Lat)))
}

// SetZoom sets the map zoom level without animation.
func (m *Map) SetZoom(z float64) {
	m.exec(fmt.Sprintf(`m.setZoom(%s);`, formatFloat(z)))
}

// SetBearing sets the map bearing without animation.
func (m *Map) SetBearing(b float64) {
	m.exec(fmt.Sprintf(`m.setBearing(%s);`, formatFloat(b)))
}

// SetPitch sets the map pitch without animation.
func (m *Map) SetPitch(p float64) {
	m.exec(fmt.Sprintf(`m.setPitch(%s);`, formatFloat(p)))
}

// SetStyle changes the map's style URL.
func (m *Map) SetStyle(url string) {
	m.exec(fmt.Sprintf(`m.setStyle(%s);`, jsonStr(url)))
}

// --- Source methods ---

// AddSource adds a source to the map.
func (m *Map) AddSource(id string, src Source) {
	js := src.sourceJS()
	if !m.rendered {
		m.sources = append(m.sources, sourceEntry{id: id, js: js})
		return
	}
	m.exec(fmt.Sprintf(`m.addSource(%s,%s);`, jsonStr(id), js))
}

// RemoveSource removes a source from the map.
func (m *Map) RemoveSource(id string) {
	if !m.rendered {
		for i, s := range m.sources {
			if s.id == id {
				m.sources = append(m.sources[:i], m.sources[i+1:]...)
				return
			}
		}
		return
	}
	m.exec(fmt.Sprintf(`m.removeSource(%s);`, jsonStr(id)))
}

// UpdateGeoJSONSource replaces the data of an existing GeoJSON source.
func (m *Map) UpdateGeoJSONSource(sourceID string, data any) {
	m.exec(fmt.Sprintf(`m.getSource(%s).setData(%s);`, jsonStr(sourceID), jsonVal(data)))
}

// --- Layer methods ---

// AddLayer adds a layer to the map.
func (m *Map) AddLayer(layer Layer) {
	if !m.rendered {
		m.layers = append(m.layers, layer)
		return
	}
	before := "undefined"
	if layer.Before != "" {
		before = jsonStr(layer.Before)
	}
	m.exec(fmt.Sprintf(`m.addLayer(%s,%s);`, layer.toJS(), before))
}

// RemoveLayer removes a layer from the map.
func (m *Map) RemoveLayer(id string) {
	if !m.rendered {
		for i, l := range m.layers {
			if l.ID == id {
				m.layers = append(m.layers[:i], m.layers[i+1:]...)
				return
			}
		}
		return
	}
	m.exec(fmt.Sprintf(`m.removeLayer(%s);`, jsonStr(id)))
}

// SetPaintProperty sets a paint property on a layer.
func (m *Map) SetPaintProperty(layerID, name string, value any) {
	m.exec(fmt.Sprintf(`m.setPaintProperty(%s,%s,%s);`,
		jsonStr(layerID), jsonStr(name), jsonVal(value)))
}

// SetLayoutProperty sets a layout property on a layer.
func (m *Map) SetLayoutProperty(layerID, name string, value any) {
	m.exec(fmt.Sprintf(`m.setLayoutProperty(%s,%s,%s);`,
		jsonStr(layerID), jsonStr(name), jsonVal(value)))
}

// --- Marker methods ---

// AddMarker adds or replaces a marker on the map.
// The returned MarkerHandle can be used to subscribe to marker-level events.
func (m *Map) AddMarker(id string, marker Marker) *MarkerHandle {
	h := &MarkerHandle{markerID: id, m: m}
	if !m.rendered {
		m.markers = append(m.markers, markerEntry{id: id, marker: marker, handle: h})
		return h
	}
	js := addMarkerJS(m.id, id, marker, h.events)
	m.ctx.ExecScript(js)
	return h
}

// OnClick returns a MapEvent that fires when this marker is clicked.
func (h *MarkerHandle) OnClick() *MapEvent {
	return h.on("click")
}

// OnDragStart returns a MapEvent that fires when dragging starts.
func (h *MarkerHandle) OnDragStart() *MapEvent {
	return h.on("dragstart")
}

// OnDrag returns a MapEvent that fires during dragging.
func (h *MarkerHandle) OnDrag() *MapEvent {
	return h.on("drag")
}

// OnDragEnd returns a MapEvent that fires when dragging ends.
func (h *MarkerHandle) OnDragEnd() *MapEvent {
	return h.on("dragend")
}

func (h *MarkerHandle) on(event string) *MapEvent {
	sig := h.m.ctx.Signal("")
	ev := &MapEvent{signal: sig}
	h.events = append(h.events, markerEventEntry{event: event, signal: sig})
	return ev
}

// RemoveMarker removes a marker from the map.
func (m *Map) RemoveMarker(id string) {
	if !m.rendered {
		for i, me := range m.markers {
			if me.id == id {
				m.markers = append(m.markers[:i], m.markers[i+1:]...)
				return
			}
		}
		return
	}
	m.exec(removeMarkerJS(id))
}

// --- Popup methods ---

// ShowPopup shows a standalone popup on the map.
// The returned PopupHandle can be used to subscribe to popup events.
func (m *Map) ShowPopup(id string, popup Popup) *PopupHandle {
	ph := &PopupHandle{popupID: id, m: m}
	if !m.rendered {
		m.popups = append(m.popups, popupEntry{id: id, popup: popup, handle: ph})
		return ph
	}
	js := showPopupJS(m.id, id, popup, ph.events)
	m.ctx.ExecScript(js)
	return ph
}

// OnOpen returns a MapEvent that fires when the popup opens.
func (ph *PopupHandle) OnOpen() *MapEvent {
	return ph.on("open")
}

// OnClose returns a MapEvent that fires when the popup closes.
func (ph *PopupHandle) OnClose() *MapEvent {
	return ph.on("close")
}

func (ph *PopupHandle) on(event string) *MapEvent {
	sig := ph.m.ctx.Signal("")
	ev := &MapEvent{signal: sig}
	ph.events = append(ph.events, popupEventEntry{event: event, signal: sig})
	return ev
}

// ClosePopup closes a standalone popup on the map.
func (m *Map) ClosePopup(id string) {
	if !m.rendered {
		for i, pe := range m.popups {
			if pe.id == id {
				m.popups = append(m.popups[:i], m.popups[i+1:]...)
				return
			}
		}
		return
	}
	m.exec(closePopupJS(id))
}

// --- Control methods ---

// AddControl adds a control to the map.
func (m *Map) AddControl(id string, ctrl Control) {
	if !m.rendered {
		m.controls = append(m.controls, controlEntry{id: id, ctrl: ctrl})
		return
	}
	m.exec(addControlJS(m.id, id, ctrl))
}

// RemoveControl removes a control from the map.
func (m *Map) RemoveControl(id string) {
	if !m.rendered {
		for i, ce := range m.controls {
			if ce.id == id {
				m.controls = append(m.controls[:i], m.controls[i+1:]...)
				return
			}
		}
		return
	}
	m.exec(removeControlJS(id))
}

// --- Event methods ---

// OnClick returns a MapEvent that fires on map click.
func (m *Map) OnClick() *MapEvent {
	return m.on("click", "")
}

// OnLayerClick returns a MapEvent that fires on click of a specific layer.
func (m *Map) OnLayerClick(layerID string) *MapEvent {
	return m.on("click", layerID)
}

// OnMouseMove returns a MapEvent that fires on map mouse movement.
func (m *Map) OnMouseMove() *MapEvent {
	return m.on("mousemove", "")
}

// OnContextMenu returns a MapEvent that fires on right-click.
func (m *Map) OnContextMenu() *MapEvent {
	return m.on("contextmenu", "")
}

func (m *Map) on(event, layerID string) *MapEvent {
	sig := m.ctx.Signal("")
	ev := &MapEvent{signal: sig}
	m.events = append(m.events, eventEntry{
		event:   event,
		layerID: layerID,
		signal:  sig,
	})
	return ev
}

// --- Escape hatch ---

// Exec runs arbitrary JS with the map available as `m`.
func (m *Map) Exec(js string) {
	m.exec(js)
}

// exec sends guarded JS to the browser via ExecScript.
func (m *Map) exec(body string) {
	m.ctx.ExecScript(guard(m.id, body))
}

// --- MapEvent ---

// MapEvent wraps a signal that receives map event data as JSON.
type MapEvent struct {
	signal *via.Signal
}

// Bind returns the data-bind attribute for this event's signal.
func (e *MapEvent) Bind() h.H { return e.signal.Bind() }

// Data parses the event signal's JSON value into EventData.
func (e *MapEvent) Data() EventData {
	var d EventData
	json.Unmarshal([]byte(e.signal.String()), &d)
	return d
}

// Input creates a hidden input wired to this event's signal.
// Pass action triggers (e.g. handleClick.OnInput()) as attrs.
func (e *MapEvent) Input(attrs ...h.H) h.H {
	all := append([]h.H{h.Type("hidden"), e.Bind()}, attrs...)
	return h.Input(all...)
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func genID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
