package main

import (
	"fmt"

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

		// Markers with popups
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

		// Viewport info signal (updated on action)
		viewportInfo := c.Signal("")

		// FlyTo action
		flyToSF := c.Action(func() {
			m.FlyTo(maplibre.LngLat{Lng: -122.4194, Lat: 37.7749}, 14)
		})

		flyToOak := c.Action(func() {
			m.FlyTo(maplibre.LngLat{Lng: -122.2711, Lat: 37.8044}, 14)
		})

		// Read viewport action
		readViewport := c.Action(func() {
			center := m.Center()
			zoom := m.Zoom()
			viewportInfo.SetValue(fmt.Sprintf("Center: %.4f, %.4f | Zoom: %.1f", center.Lng, center.Lat, zoom))
			c.Sync()
		})

		c.View(func() h.H {
			return h.Div(
				h.Div(
					h.Attr("style", "max-width:960px;margin:0 auto;padding:1rem;font-family:sans-serif"),
					h.H1(h.Text("MapLibre GL Example")),
					m.Element(),
					h.Div(h.Attr("style", "margin-top:1rem;display:flex;gap:0.5rem"),
						h.Button(h.Text("Fly to San Francisco"), flyToSF.OnClick()),
						h.Button(h.Text("Fly to Oakland"), flyToOak.OnClick()),
						h.Button(h.Text("Read Viewport"), readViewport.OnClick()),
					),
					h.P(viewportInfo.Text()),
				),
			)
		})
	})

	v.Start()
}
