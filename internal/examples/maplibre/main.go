package main

import (
	"fmt"
	"math"
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

// shipState tracks a ship lerping along a loop of waypoints.
type shipState struct {
	lng, lat  float64
	waypoints [][2]float64 // [lng, lat] pairs
	wpIdx     int          // index of next target waypoint
	progress  float64      // 0..1 toward next waypoint
	speed     float64      // progress increment per tick
}

func (s *shipState) tick() {
	s.progress += s.speed
	for s.progress >= 1 {
		s.progress -= 1
		s.wpIdx = (s.wpIdx + 1) % len(s.waypoints)
	}
	from := s.waypoints[(s.wpIdx-1+len(s.waypoints))%len(s.waypoints)]
	to := s.waypoints[s.wpIdx]
	s.lng = from[0] + (to[0]-from[0])*s.progress
	s.lat = from[1] + (to[1]-from[1])*s.progress
}

// heading returns clockwise degrees from north (for SVG rotation).
func (s *shipState) heading() float64 {
	from := s.waypoints[(s.wpIdx-1+len(s.waypoints))%len(s.waypoints)]
	to := s.waypoints[s.wpIdx]
	dx := to[0] - from[0]
	dy := to[1] - from[1]
	// atan2 gives angle from +X axis; convert to CW from north
	return math.Mod(math.Atan2(dx, dy)*180/math.Pi+360, 360)
}

var (
	fleetOnce sync.Once
	fleet     struct {
		mu    sync.RWMutex
		ships [3]shipState
	}
)

const shipSVG = `<svg width="48" height="28" viewBox="0 0 80 44" xmlns="http://www.w3.org/2000/svg">` +
	`<path d="M2 30 L10 42 L70 42 L78 30 Z" fill="#1b3a5c"/>` +
	`<rect x="12" y="24" width="56" height="6" rx="1" fill="#2c5f8a"/>` +
	`<rect x="18" y="14" width="46" height="10" rx="1" fill="#2c5f8a" stroke="#1b3a5c" stroke-width="0.5"/>` +
	`<rect x="8" y="12" width="9" height="12" rx="1" fill="#d5dfe8" stroke="#2c5f8a" stroke-width="0.5"/>` +
	`<rect x="9.5" y="13" width="6" height="4" rx="0.5" fill="#85c1e9"/>` +
	`<rect x="10" y="4" width="5" height="8" rx="0.5" fill="#1b3a5c"/>` +
	`<rect x="9.5" y="2" width="6" height="3" rx="0.5" fill="#c0392b"/>` +
	`</svg>`

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

	// Fleet of ships following waypoint loops through SF Bay.
	fleetOnce.Do(func() {
		fleet.ships = [3]shipState{
			{ // Golden Gate → Alcatraz → Pier 39 → back out
				waypoints: [][2]float64{
					{-122.478, 37.819}, {-122.423, 37.827},
					{-122.410, 37.809}, {-122.423, 37.827},
				},
				speed: 0.03,
			},
			{ // Oakland → Treasure Island → Angel Island → loop
				waypoints: [][2]float64{
					{-122.330, 37.795}, {-122.370, 37.823},
					{-122.432, 37.860}, {-122.370, 37.823},
				},
				speed: 0.02,
			},
			{ // Sausalito → Pier 39 ferry route
				waypoints: [][2]float64{
					{-122.480, 37.859}, {-122.435, 37.840},
					{-122.410, 37.809}, {-122.435, 37.840},
				},
				speed: 0.025,
			},
		}
		// Set initial positions.
		for i := range fleet.ships {
			s := &fleet.ships[i]
			s.lng = s.waypoints[0][0]
			s.lat = s.waypoints[0][1]
			s.wpIdx = 1
		}
		go func() {
			for {
				time.Sleep(time.Second)
				fleet.mu.Lock()
				for i := range fleet.ships {
					fleet.ships[i].tick()
				}
				fleet.mu.Unlock()
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

		// Animated container ships following waypoint routes
		shipNames := [3]string{"MSC Adriatica", "Evergreen Harmony", "Maersk Aurora"}
		type shipSignals struct{ lng, lat, rot *via.Signal }
		var ships [3]shipSignals

		fleet.mu.RLock()
		for i, s := range fleet.ships {
			ships[i].lng = c.Signal(s.lng)
			ships[i].lat = c.Signal(s.lat)
			// SVG bow points right (east), so subtract 90° from the north-based heading.
			ships[i].rot = c.Signal(s.heading() - 90)
			m.AddMarker(fmt.Sprintf("ship-%d", i), maplibre.Marker{
				LngSignal:      ships[i].lng,
				LatSignal:      ships[i].lat,
				RotationSignal: ships[i].rot,
				Element:        shipSVG,
				Anchor:         "center",
				Popup: &maplibre.Popup{
					Content: fmt.Sprintf("<strong>%s</strong>", shipNames[i]),
				},
			})
		}
		fleet.mu.RUnlock()

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

			fleet.mu.RLock()
			for i, s := range fleet.ships {
				ships[i].lng.SetValue(s.lng)
				ships[i].lat.SetValue(s.lat)
				ships[i].rot.SetValue(s.heading() - 90)
			}
			fleet.mu.RUnlock()

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
