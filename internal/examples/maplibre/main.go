package main

import (
	"math/rand"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
	"github.com/ryanhamamura/via/maplibre"
)

func main() {
	v := via.New()
	v.Config(via.Options{
		DocumentTitle: "MapLibre GL Example",
		ServerAddress: ":7331",
		DevMode:       true,
		Plugins:       []via.Plugin{maplibre.Plugin},
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

		// Signal-backed marker — server pushes position updates
		vehicleLng := c.Signal(-122.43)
		vehicleLat := c.Signal(37.77)

		m.AddMarker("vehicle", maplibre.Marker{
			LngSignal: vehicleLng,
			LatSignal: vehicleLat,
			Color:     "#9b59b6",
		})

		c.OnInterval(time.Second, func() {
			vehicleLng.SetValue(-122.43 + (rand.Float64()-0.5)*0.02)
			vehicleLat.SetValue(37.77 + (rand.Float64()-0.5)*0.02)
			c.SyncSignals()
		})

		// Draggable marker — user drags, signals update
		pinLng := c.Signal(-122.41)
		pinLat := c.Signal(37.78)

		m.AddMarker("pin", maplibre.Marker{
			LngSignal: pinLng,
			LatSignal: pinLat,
			Color:     "#3498db",
			Draggable: true,
		})

		// Click event — click to place a marker
		click := m.OnClick()
		handleClick := c.Action(func() {
			e := click.Data()
			m.AddMarker("clicked", maplibre.Marker{
				LngLat: e.LngLat,
				Color:  "#f39c12",
			})
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

		// FlyTo actions using CameraOptions
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
					),
					h.Div(h.Attr("style", "margin-top:1rem;display:flex;gap:0.5rem;flex-wrap:wrap"),
						h.Button(h.Text("Fly to San Francisco"), flyToSF.OnClick()),
						h.Button(h.Text("Fly to Oakland"), flyToOak.OnClick()),
					),
					h.Div(h.Attr("style", "margin-top:0.5rem;font-size:0.9rem"),
						h.P(h.Text("Zoom: "), m.Zoom.Text()),
						h.P(h.Text("Center: "), m.CenterLng.Text(), h.Text(", "), m.CenterLat.Text()),
						h.P(h.Text("Vehicle: "), vehicleLng.Text(), h.Text(", "), vehicleLat.Text()),
						h.P(h.Text("Draggable Pin: "), pinLng.Text(), h.Text(", "), pinLat.Text()),
					),
				),
			)
		})
	})

	v.Start()
}
