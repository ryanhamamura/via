package maplibre

import "encoding/json"

// LngLat represents a geographic coordinate.
type LngLat struct {
	Lng float64
	Lat float64
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
}

// GeoJSONSource provides inline GeoJSON data to MapLibre.
// Data should be a GeoJSON-marshalable value (struct, map, or json.RawMessage).
type GeoJSONSource struct {
	Data any
}

func (s GeoJSONSource) toJS() string {
	data, _ := json.Marshal(s.Data)
	return `{"type":"geojson","data":` + string(data) + `}`
}

// VectorSource references a vector tile source.
type VectorSource struct {
	URL   string
	Tiles []string
}

func (s VectorSource) toJS() string {
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

func (s RasterSource) toJS() string {
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

// sourceJSON converts a source value to its JS object literal string.
func sourceJSON(src any) string {
	switch s := src.(type) {
	case GeoJSONSource:
		return s.toJS()
	case VectorSource:
		return s.toJS()
	case RasterSource:
		return s.toJS()
	default:
		b, _ := json.Marshal(src)
		return string(b)
	}
}

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

// Marker describes a map marker.
type Marker struct {
	LngLat    LngLat
	Color     string
	Draggable bool
	Popup     *Popup
}

// Popup describes a map popup.
//
// Content is rendered as HTML via MapLibre's setHTML. Do not pass untrusted
// user input without sanitizing it first.
type Popup struct {
	Content         string // HTML content
	LngLat          LngLat
	HideCloseButton bool   // true removes the close button (MapLibre shows it by default)
	MaxWidth        string
}

// sourceEntry pairs a source ID with its JS representation for pre-render accumulation.
type sourceEntry struct {
	id string
	js string
}

// markerEntry pairs a marker ID with its definition for pre-render accumulation.
type markerEntry struct {
	id     string
	marker Marker
}

// popupEntry pairs a popup ID with its definition for pre-render accumulation.
type popupEntry struct {
	id    string
	popup Popup
}
