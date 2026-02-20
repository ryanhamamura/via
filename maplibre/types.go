package maplibre

import (
	"encoding/json"

	"github.com/ryanhamamura/via"
)

// LngLat represents a geographic coordinate.
type LngLat struct {
	Lng float64
	Lat float64
}

// LngLatBounds represents a rectangular geographic area.
type LngLatBounds struct {
	SW LngLat
	NE LngLat
}

// Padding represents padding in pixels on each side of the map viewport.
type Padding struct {
	Top    int
	Bottom int
	Left   int
	Right  int
}

// Options configures the initial map state.
type Options struct {
	// Style is the map style URL (required).
	Style string

	Center  LngLat
	Zoom    float64
	Bearing float64
	Pitch   float64
	MinZoom float64
	MaxZoom float64

	// CSS dimensions for the map container. Defaults: "100%", "400px".
	Width  string
	Height string

	// Interaction toggles (nil = MapLibre default)
	ScrollZoom        *bool
	BoxZoom           *bool
	DragRotate        *bool
	DragPan           *bool
	Keyboard          *bool
	DoubleClickZoom   *bool
	TouchZoomRotate   *bool
	TouchPitch        *bool
	RenderWorldCopies *bool

	MaxBounds *LngLatBounds

	// Extra is merged last into the MapLibre constructor options object,
	// allowing pass-through of any option not covered above.
	Extra map[string]any
}

// --- Source interface ---

// Source is implemented by map data sources (GeoJSON, vector, raster, etc.).
type Source interface {
	sourceJS() string
}

// GeoJSONSource provides inline GeoJSON data to MapLibre.
// Data should be a GeoJSON-marshalable value (struct, map, or json.RawMessage).
type GeoJSONSource struct {
	Data any
}

func (s GeoJSONSource) sourceJS() string {
	data, _ := json.Marshal(s.Data)
	return `{"type":"geojson","data":` + string(data) + `}`
}

// VectorSource references a vector tile source.
type VectorSource struct {
	URL   string
	Tiles []string
}

func (s VectorSource) sourceJS() string {
	obj := map[string]any{"type": "vector"}
	if s.URL != "" {
		obj["url"] = s.URL
	}
	if len(s.Tiles) > 0 {
		obj["tiles"] = s.Tiles
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// RasterSource references a raster tile source.
type RasterSource struct {
	URL      string
	Tiles    []string
	TileSize int
}

func (s RasterSource) sourceJS() string {
	obj := map[string]any{"type": "raster"}
	if s.URL != "" {
		obj["url"] = s.URL
	}
	if len(s.Tiles) > 0 {
		obj["tiles"] = s.Tiles
	}
	if s.TileSize > 0 {
		obj["tileSize"] = s.TileSize
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// RawSource is an escape hatch that passes an arbitrary JSON-marshalable
// value directly as a MapLibre source definition.
type RawSource struct {
	Value any
}

func (s RawSource) sourceJS() string {
	b, _ := json.Marshal(s.Value)
	return string(b)
}

// --- Control interface ---

// Control is implemented by map controls (navigation, scale, etc.).
type Control interface {
	controlJS() string
	controlPosition() string
}

// NavigationControl adds zoom and rotation buttons.
type NavigationControl struct {
	Position       string // "top-right" (default), "top-left", "bottom-right", "bottom-left"
	ShowCompass    *bool
	ShowZoom       *bool
	VisualizeRoll  *bool
	VisualizePitch *bool
}

func (c NavigationControl) controlJS() string {
	opts := map[string]any{}
	if c.ShowCompass != nil {
		opts["showCompass"] = *c.ShowCompass
	}
	if c.ShowZoom != nil {
		opts["showZoom"] = *c.ShowZoom
	}
	if c.VisualizeRoll != nil {
		opts["visualizeRoll"] = *c.VisualizeRoll
	}
	if c.VisualizePitch != nil {
		opts["visualizePitch"] = *c.VisualizePitch
	}
	b, _ := json.Marshal(opts)
	return "new maplibregl.NavigationControl(" + string(b) + ")"
}

func (c NavigationControl) controlPosition() string {
	if c.Position == "" {
		return "top-right"
	}
	return c.Position
}

// ScaleControl displays a scale bar.
type ScaleControl struct {
	Position string // default "bottom-left"
	MaxWidth int
	Unit     string // "metric", "imperial", "nautical"
}

func (c ScaleControl) controlJS() string {
	opts := map[string]any{}
	if c.MaxWidth > 0 {
		opts["maxWidth"] = c.MaxWidth
	}
	if c.Unit != "" {
		opts["unit"] = c.Unit
	}
	b, _ := json.Marshal(opts)
	return "new maplibregl.ScaleControl(" + string(b) + ")"
}

func (c ScaleControl) controlPosition() string {
	if c.Position == "" {
		return "bottom-left"
	}
	return c.Position
}

// GeolocateControl adds a button to track the user's location.
type GeolocateControl struct {
	Position string // default "top-right"
}

func (c GeolocateControl) controlJS() string {
	return "new maplibregl.GeolocateControl()"
}

func (c GeolocateControl) controlPosition() string {
	if c.Position == "" {
		return "top-right"
	}
	return c.Position
}

// FullscreenControl adds a fullscreen toggle button.
type FullscreenControl struct {
	Position string // default "top-right"
}

func (c FullscreenControl) controlJS() string {
	return "new maplibregl.FullscreenControl()"
}

func (c FullscreenControl) controlPosition() string {
	if c.Position == "" {
		return "top-right"
	}
	return c.Position
}

// --- Camera options ---

// CameraOptions configures animated camera movements (FlyTo, EaseTo, JumpTo).
// Nil pointer fields are omitted from the JS call.
type CameraOptions struct {
	Center   *LngLat
	Zoom     *float64
	Bearing  *float64
	Pitch    *float64
	Duration *int     // milliseconds
	Speed    *float64 // FlyTo only
	Curve    *float64 // FlyTo only
	Padding  *Padding
	Animate  *bool
}

// --- Layer ---

// Layer describes a MapLibre style layer.
type Layer struct {
	ID          string
	Type        string
	Source      string
	SourceLayer string
	Paint       map[string]any
	Layout      map[string]any
	Filter      any
	MinZoom     float64
	MaxZoom     float64

	// Before inserts this layer before the given layer ID in the stack.
	Before string
}

func (l Layer) toJS() string {
	obj := map[string]any{
		"id":   l.ID,
		"type": l.Type,
	}
	if l.Source != "" {
		obj["source"] = l.Source
	}
	if l.SourceLayer != "" {
		obj["source-layer"] = l.SourceLayer
	}
	if l.Paint != nil {
		obj["paint"] = l.Paint
	}
	if l.Layout != nil {
		obj["layout"] = l.Layout
	}
	if l.Filter != nil {
		obj["filter"] = l.Filter
	}
	if l.MinZoom > 0 {
		obj["minzoom"] = l.MinZoom
	}
	if l.MaxZoom > 0 {
		obj["maxzoom"] = l.MaxZoom
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// --- Marker ---

// Marker describes a map marker.
type Marker struct {
	LngLat    LngLat // static position (used when signals are nil)
	Color     string
	Draggable bool
	Popup     *Popup

	// Element is raw HTML/SVG used as a custom marker instead of the
	// default pin. When set, Color is ignored.
	// Do not pass untrusted user input without sanitizing it first.
	Element string

	// Anchor controls which part of the element sits at the coordinate.
	// Values: "center" (default for custom elements), "bottom" (default
	// for the pin), "top", "left", "right", "top-left", etc.
	Anchor string

	// Rotation is clockwise degrees. Useful for directional icons (ships, vehicles).
	Rotation float64

	// Signal-backed position. When set, signals drive marker position reactively.
	// Initial position is read from the signal values. LngLat is ignored when signals are set.
	// If Draggable is true, drag updates write back to these signals.
	LngSignal *via.Signal
	LatSignal *via.Signal
}

// Popup describes a map popup.
//
// Content is rendered as HTML via MapLibre's setHTML. Do not pass untrusted
// user input without sanitizing it first.
type Popup struct {
	Content         string // HTML content
	LngLat          LngLat
	HideCloseButton bool // true removes the close button (MapLibre shows it by default)
	MaxWidth        string
}

// --- Event data ---

// EventData contains data from a map event (click, mousemove, etc.).
type EventData struct {
	LngLat   LngLat            `json:"lngLat"`
	Point    [2]float64        `json:"point"`
	Features []json.RawMessage `json:"features,omitempty"`
	LayerID  string            `json:"layerID,omitempty"`
}

// --- Internal accumulation entries ---

type sourceEntry struct {
	id string
	js string
}

type markerEntry struct {
	id     string
	marker Marker
}

type popupEntry struct {
	id    string
	popup Popup
}

type eventEntry struct {
	event   string
	layerID string
	signal  *via.Signal
}

type controlEntry struct {
	id   string
	ctrl Control
}
