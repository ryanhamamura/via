package main

import (
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
	"github.com/ryanhamamura/via/maplibre"
)

type posMsg struct {
	Lng float64 `json:"lng"`
	Lat float64 `json:"lat"`
}

var (
	vehicleOnce sync.Once
	vehicle     struct {
		mu       sync.RWMutex
		lng, lat float64
	}
)

func main() {
	v := via.New()
	v.Config(via.Options{
		DocumentTitle: "MapLibre GL Example",
		ServerAddress: ":7331",
		DevMode:       true,
		Plugins:       []via.Plugin{maplibre.Plugin},
	})

	// Single goroutine moves the vehicle — all clients read the same position.
	vehicle.lng = -122.43
	vehicle.lat = 37.77
	vehicleOnce.Do(func() {
		go func() {
			for {
				time.Sleep(time.Second)
				vehicle.mu.Lock()
				vehicle.lng = -122.43 + (rand.Float64()-0.5)*0.02
				vehicle.lat = 37.77 + (rand.Float64()-0.5)*0.02
				vehicle.mu.Unlock()
			}
		}()
	})

	v.Page("/", func(c *via.Context) {
		m := maplibre.New(c, maplibre.Options{
			Style:  "https://demotiles.maplibre.org/style.json",
			Center: maplibre.LngLat{Lng: -122.4194, Lat: 37.7749},
			Zoom:   10,
			Height: "500px",
		})

		m.AddControl("nav", maplibre.NavigationControl{})
		m.AddControl("scale", maplibre.ScaleControl{Unit: "metric"})

		// Static markers with popups
		m.AddMarker("sf", maplibre.Marker{
			LngLat: maplibre.LngLat{Lng: -122.4194, Lat: 37.7749},
			Color:  "#e74c3c",
			Popup: &maplibre.Popup{
				Content: "<strong>San Francisco</strong><p>The Golden City</p>",
			},
		})
		m.AddMarker("oak", maplibre.Marker{
			LngLat: maplibre.LngLat{Lng: -122.2711, Lat: 37.8044},
			Color:  "#2ecc71",
			Popup: &maplibre.Popup{
				Content: "<strong>Oakland</strong>",
			},
		})

		// Custom SVG container ship marker (static)
		m.AddMarker("ship", maplibre.Marker{
			LngLat: maplibre.LngLat{Lng: -122.38, Lat: 37.80},
			Element: `<svg width="48" height="28" viewBox="0 0 80 44" xmlns="http://www.w3.org/2000/svg">` +
				`<path d="M2 30 L10 42 L70 42 L78 30 Z" fill="#1b3a5c"/>` +
				`<rect x="12" y="24" width="56" height="6" rx="1" fill="#2c5f8a"/>` +
				`<rect x="18" y="14" width="46" height="10" rx="1" fill="#2c5f8a" stroke="#1b3a5c" stroke-width="0.5"/>` +
				`<rect x="8" y="12" width="9" height="12" rx="1" fill="#d5dfe8" stroke="#2c5f8a" stroke-width="0.5"/>` +
				`<rect x="9.5" y="13" width="6" height="4" rx="0.5" fill="#85c1e9"/>` +
				`<rect x="10" y="4" width="5" height="8" rx="0.5" fill="#1b3a5c"/>` +
				`<rect x="9.5" y="2" width="6" height="3" rx="0.5" fill="#c0392b"/>` +
				`</svg>`,
			Anchor:   "center",
			Rotation: 45,
			Popup: &maplibre.Popup{
				Content: "<strong>MSC Adriatica</strong><p>Container vessel — heading NE</p>",
			},
		})

		// Custom SVG vehicle marker — reads shared Go state
		vehicleLng := c.Signal(-122.43)
		vehicleLat := c.Signal(37.77)

		m.AddMarker("vehicle", maplibre.Marker{
			LngSignal: vehicleLng,
			LatSignal: vehicleLat,
			Element: `<svg width="20" height="20" viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">` +
				`<circle cx="10" cy="10" r="9" fill="#9b59b6" stroke="#fff" stroke-width="2"/>` +
				`</svg>`,
			Anchor: "center",
		})

		c.OnInterval(time.Second, func() {
			vehicle.mu.RLock()
			lng, lat := vehicle.lng, vehicle.lat
			vehicle.mu.RUnlock()
			vehicleLng.SetValue(lng)
			vehicleLat.SetValue(lat)
			c.SyncSignals()
		})

		// Yellow click marker — synced across clients via PubSub
		clickLng := c.Signal(-122.4194)
		clickLat := c.Signal(37.7749)

		m.AddMarker("clicked", maplibre.Marker{
			LngSignal: clickLng,
			LatSignal: clickLat,
			Color:     "#f39c12",
		})

		via.Subscribe(c, "map.click", func(msg posMsg) {
			clickLng.SetValue(msg.Lng)
			clickLat.SetValue(msg.Lat)
			c.SyncSignals()
		})

		click := m.OnClick()
		handleClick := c.Action(func() {
			e := click.Data()
			via.Publish(c, "map.click", posMsg{Lng: e.LngLat.Lng, Lat: e.LngLat.Lat})
		})

		// Blue draggable pin — synced across clients via PubSub
		pinLng := c.Signal(-122.41)
		pinLat := c.Signal(37.78)

		m.AddMarker("pin", maplibre.Marker{
			LngSignal: pinLng,
			LatSignal: pinLat,
			Color:     "#3498db",
			Draggable: true,
		})

		via.Subscribe(c, "map.pin", func(msg posMsg) {
			pinLng.SetValue(msg.Lng)
			pinLat.SetValue(msg.Lat)
			c.SyncSignals()
		})

		handlePinDrag := c.Action(func() {
			lng, _ := strconv.ParseFloat(pinLng.String(), 64)
			lat, _ := strconv.ParseFloat(pinLat.String(), 64)
			via.Publish(c, "map.pin", posMsg{Lng: lng, Lat: lat})
		})

		// GeoJSON polygon source + fill layer
		m.AddSource("park", maplibre.GeoJSONSource{
			Data: map[string]any{
				"type": "Feature",
				"geometry": map[string]any{
					"type": "Polygon",
					"coordinates": []any{[]any{
						[]float64{-122.4547, 37.7654},
						[]float64{-122.4547, 37.7754},
						[]float64{-122.4387, 37.7754},
						[]float64{-122.4387, 37.7654},
						[]float64{-122.4547, 37.7654},
					}},
				},
				"properties": map[string]any{
					"name": "Golden Gate Park",
				},
			},
		})
		m.AddLayer(maplibre.Layer{
			ID:     "park-fill",
			Type:   "fill",
			Source: "park",
			Paint: map[string]any{
				"fill-color":   "#2ecc71",
				"fill-opacity": 0.3,
			},
		})

		// FlyTo actions
		zoom14 := 14.0
		flyToSF := c.Action(func() {
			m.FlyTo(maplibre.CameraOptions{
				Center: &maplibre.LngLat{Lng: -122.4194, Lat: 37.7749},
				Zoom:   &zoom14,
			})
		})

		flyToOak := c.Action(func() {
			m.FlyTo(maplibre.CameraOptions{
				Center: &maplibre.LngLat{Lng: -122.2711, Lat: 37.8044},
				Zoom:   &zoom14,
			})
		})

		c.View(func() h.H {
			return h.Div(
				h.Div(
					h.Attr("style", "max-width:960px;margin:0 auto;padding:1rem;font-family:sans-serif"),
					h.H1(h.Text("MapLibre GL Example")),
					m.Element(
						click.Input(handleClick.OnInput()),
						h.Input(h.Type("hidden"), pinLng.Bind(), handlePinDrag.OnInput()),
					),
					h.Div(h.Attr("style", "margin-top:1rem;display:flex;gap:0.5rem;flex-wrap:wrap"),
						h.Button(h.Text("Fly to San Francisco"), flyToSF.OnClick()),
						h.Button(h.Text("Fly to Oakland"), flyToOak.OnClick()),
					),
					h.Div(h.Attr("style", "margin-top:0.5rem;font-size:0.9rem"),
						h.P(h.Text("Zoom: "), m.Zoom.Text()),
						h.P(h.Text("Center: "), m.CenterLng.Text(), h.Text(", "), m.CenterLat.Text()),
						h.P(h.Text("Click: "), clickLng.Text(), h.Text(", "), clickLat.Text()),
						h.P(h.Text("Vehicle: "), vehicleLng.Text(), h.Text(", "), vehicleLat.Text()),
						h.P(h.Text("Draggable Pin: "), pinLng.Text(), h.Text(", "), pinLat.Text()),
					),
				),
			)
		})
	})

	v.Start()
}
