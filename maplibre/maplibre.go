// Package maplibre provides a Go API for MapLibre GL JS maps within Via applications.
//
// It follows the same ExecScript + DataIgnoreMorph pattern used for other client-side
// JS library integrations (e.g. ECharts in the realtimechart example).
package maplibre

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
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

// viaSignal is the interface satisfied by via's *signal type.
type viaSignal interface {
	ID() string
	String() string
	SetValue(any)
	Bind() h.H
}

// Map represents a MapLibre GL map instance bound to a Via context.
type Map struct {
	id   string
	ctx  *via.Context
	opts Options

	// Viewport signals for browser → server sync
	centerLng, centerLat viaSignal
	zoom, bearing, pitch viaSignal

	// Pre-render accumulation
	sources []sourceEntry
	layers  []Layer
	markers []markerEntry
	popups  []popupEntry

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

	m.centerLng = c.Signal(opts.Center.Lng)
	m.centerLat = c.Signal(opts.Center.Lat)
	m.zoom = c.Signal(opts.Zoom)
	m.bearing = c.Signal(opts.Bearing)
	m.pitch = c.Signal(opts.Pitch)

	return m
}

// Element returns the h.H DOM tree for the map. Call this once inside your View function.
// After Element() is called, subsequent source/layer/marker/popup operations
// use ExecScript instead of accumulating for the init script.
func (m *Map) Element() h.H {
	m.rendered = true

	return h.Div(h.ID("_vwrap_"+m.id),
		// Map container — morph-ignored so MapLibre's DOM isn't destroyed on Sync()
		h.Div(
			h.ID("_vmap_"+m.id),
			h.DataIgnoreMorph(),
			h.Attr("style", fmt.Sprintf("width:%s;height:%s", m.opts.Width, m.opts.Height)),
		),
		// Hidden inputs for viewport signal binding (outside morph-ignored zone)
		h.Input(h.Type("hidden"), m.centerLng.Bind()),
		h.Input(h.Type("hidden"), m.centerLat.Bind()),
		h.Input(h.Type("hidden"), m.zoom.Bind()),
		h.Input(h.Type("hidden"), m.bearing.Bind()),
		h.Input(h.Type("hidden"), m.pitch.Bind()),
		// Init script
		h.Script(h.Raw(initScript(m))),
	)
}

// --- Viewport readers (signal → Go) ---

// Center returns the current map center from synced signals.
func (m *Map) Center() LngLat {
	return LngLat{
		Lng: parseFloat(m.centerLng.String()),
		Lat: parseFloat(m.centerLat.String()),
	}
}

// Zoom returns the current map zoom level from the synced signal.
func (m *Map) Zoom() float64 {
	return parseFloat(m.zoom.String())
}

// Bearing returns the current map bearing from the synced signal.
func (m *Map) Bearing() float64 {
	return parseFloat(m.bearing.String())
}

// Pitch returns the current map pitch from the synced signal.
func (m *Map) Pitch() float64 {
	return parseFloat(m.pitch.String())
}

// --- Viewport setters (Go → browser) ---

// FlyTo animates the map to the given center and zoom.
func (m *Map) FlyTo(center LngLat, zoom float64) {
	m.exec(fmt.Sprintf(`m.flyTo({center:[%s,%s],zoom:%s});`,
		formatFloat(center.Lng), formatFloat(center.Lat), formatFloat(zoom)))
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

// AddSource adds a source to the map. src should be a GeoJSONSource,
// VectorSource, RasterSource, or any JSON-marshalable value.
func (m *Map) AddSource(id string, src any) {
	js := sourceJSON(src)
	if !m.rendered {
		m.sources = append(m.sources, sourceEntry{id: id, js: js})
		return
	}
	m.exec(fmt.Sprintf(`m.addSource(%s,%s);`, jsonStr(id), js))
}

// RemoveSource removes a source from the map.
// Before render, it removes a previously accumulated source. After render, it issues an ExecScript.
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
// Before render, it removes a previously accumulated layer. After render, it issues an ExecScript.
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
func (m *Map) AddMarker(id string, marker Marker) {
	if !m.rendered {
		m.markers = append(m.markers, markerEntry{id: id, marker: marker})
		return
	}
	js := addMarkerJS(m.id, id, marker)
	m.ctx.ExecScript(js)
}

// RemoveMarker removes a marker from the map.
// Before render, it removes a previously accumulated marker. After render, it issues an ExecScript.
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
func (m *Map) ShowPopup(id string, popup Popup) {
	if !m.rendered {
		m.popups = append(m.popups, popupEntry{id: id, popup: popup})
		return
	}
	js := showPopupJS(m.id, id, popup)
	m.ctx.ExecScript(js)
}

// ClosePopup closes a standalone popup on the map.
// Before render, it removes a previously accumulated popup. After render, it issues an ExecScript.
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

// --- Escape hatch ---

// Exec runs arbitrary JS with the map available as `m`.
func (m *Map) Exec(js string) {
	m.exec(js)
}

// exec sends guarded JS to the browser via ExecScript.
func (m *Map) exec(body string) {
	m.ctx.ExecScript(guard(m.id, body))
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
