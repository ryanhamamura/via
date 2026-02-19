package maplibre

import (
	"encoding/json"
	"fmt"
	"strings"
)

// guard wraps JS code so it only runs when the map instance exists.
// The body can reference the map as `m`.
func guard(mapID, body string) string {
	return fmt.Sprintf(
		`(function(){var m=window.__via_maps&&window.__via_maps[%s];if(!m)return;%s})()`,
		jsonStr(mapID), body,
	)
}

// jsonStr JSON-encodes a string for safe embedding in JS.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// jsonVal JSON-encodes an arbitrary value for safe embedding in JS.
func jsonVal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// initScript generates the idempotent map initialization JS.
func initScript(m *Map) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(
		`(function(){if(window.__via_maps&&window.__via_maps[%[1]s])return;`,
		jsonStr(m.id),
	))

	b.WriteString(fmt.Sprintf(
		`var map=new maplibregl.Map({container:%s,style:%s,center:[%s,%s],zoom:%s`,
		jsonStr("_vmap_"+m.id),
		jsonStr(m.opts.Style),
		formatFloat(m.opts.Center.Lng),
		formatFloat(m.opts.Center.Lat),
		formatFloat(m.opts.Zoom),
	))
	if m.opts.Bearing != 0 {
		b.WriteString(fmt.Sprintf(`,bearing:%s`, formatFloat(m.opts.Bearing)))
	}
	if m.opts.Pitch != 0 {
		b.WriteString(fmt.Sprintf(`,pitch:%s`, formatFloat(m.opts.Pitch)))
	}
	if m.opts.MinZoom != 0 {
		b.WriteString(fmt.Sprintf(`,minZoom:%s`, formatFloat(m.opts.MinZoom)))
	}
	if m.opts.MaxZoom != 0 {
		b.WriteString(fmt.Sprintf(`,maxZoom:%s`, formatFloat(m.opts.MaxZoom)))
	}
	b.WriteString(`});`)

	b.WriteString(`if(!window.__via_maps)window.__via_maps={};`)
	b.WriteString(fmt.Sprintf(`window.__via_maps[%s]=map;`, jsonStr(m.id)))
	b.WriteString(`map._via_markers={};map._via_popups={};`)

	// Pre-render sources, layers, markers, popups run on 'load'
	if len(m.sources) > 0 || len(m.layers) > 0 || len(m.markers) > 0 || len(m.popups) > 0 {
		b.WriteString(`map.on('load',function(){`)
		for _, src := range m.sources {
			b.WriteString(fmt.Sprintf(`map.addSource(%s,%s);`, jsonStr(src.id), src.js))
		}
		for _, layer := range m.layers {
			if layer.Before != "" {
				b.WriteString(fmt.Sprintf(`map.addLayer(%s,%s);`, layer.toJS(), jsonStr(layer.Before)))
			} else {
				b.WriteString(fmt.Sprintf(`map.addLayer(%s);`, layer.toJS()))
			}
		}
		for _, me := range m.markers {
			b.WriteString(markerBodyJS(me.id, me.marker))
		}
		for _, pe := range m.popups {
			b.WriteString(popupBodyJS(pe.id, pe.popup))
		}
		b.WriteString(`});`)
	}

	// Sync viewport signals on moveend via hidden inputs
	b.WriteString(fmt.Sprintf(`map.on('moveend',function(){`+
		`var c=map.getCenter();`+
		`var el=document.getElementById(%[1]s);if(!el)return;`+
		`var inputs=el.querySelectorAll('input[data-bind]');`+
		`inputs.forEach(function(inp){`+
		`var sig=inp.getAttribute('data-bind');`+
		`if(sig===%[2]s)inp.value=c.lng;`+
		`else if(sig===%[3]s)inp.value=c.lat;`+
		`else if(sig===%[4]s)inp.value=map.getZoom();`+
		`else if(sig===%[5]s)inp.value=map.getBearing();`+
		`else if(sig===%[6]s)inp.value=map.getPitch();`+
		`inp.dispatchEvent(new Event('input',{bubbles:true}));`+
		`});`+
		`});`,
		jsonStr("_vwrap_"+m.id),
		jsonStr(m.centerLng.ID()),
		jsonStr(m.centerLat.ID()),
		jsonStr(m.zoom.ID()),
		jsonStr(m.bearing.ID()),
		jsonStr(m.pitch.ID()),
	))

	// ResizeObserver for auto-resize
	b.WriteString(fmt.Sprintf(
		`var ro=new ResizeObserver(function(){map.resize();});`+
			`ro.observe(document.getElementById(%s));`,
		jsonStr("_vmap_"+m.id),
	))

	// MutationObserver to clean up on DOM removal (SPA nav)
	b.WriteString(fmt.Sprintf(
		`var container=document.getElementById(%[1]s);`+
			`if(container){var mo=new MutationObserver(function(){`+
			`if(!document.contains(container)){`+
			`mo.disconnect();ro.disconnect();map.remove();`+
			`delete window.__via_maps[%[2]s];`+
			`}});`+
			`mo.observe(document.body,{childList:true,subtree:true});}`,
		jsonStr("_vmap_"+m.id),
		jsonStr(m.id),
	))

	b.WriteString(`})()`)
	return b.String()
}

// markerBodyJS generates JS to add a marker, assuming `map` is in scope.
// Used inside the init script's load callback.
func markerBodyJS(markerID string, mk Marker) string {
	var b strings.Builder
	opts := "{"
	if mk.Color != "" {
		opts += fmt.Sprintf(`color:%s,`, jsonStr(mk.Color))
	}
	if mk.Draggable {
		opts += `draggable:true,`
	}
	opts += "}"
	b.WriteString(fmt.Sprintf(`var mk=new maplibregl.Marker(%s).setLngLat([%s,%s]);`,
		opts, formatFloat(mk.LngLat.Lng), formatFloat(mk.LngLat.Lat)))
	if mk.Popup != nil {
		b.WriteString(popupConstructorJS(*mk.Popup, "pk"))
		b.WriteString(`mk.setPopup(pk);`)
	}
	b.WriteString(fmt.Sprintf(`mk.addTo(map);map._via_markers[%s]=mk;`, jsonStr(markerID)))
	return b.String()
}

// addMarkerJS generates a self-contained IIFE to add a marker post-render.
func addMarkerJS(mapID, markerID string, mk Marker) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		`(function(){var map=window.__via_maps&&window.__via_maps[%s];if(!map)return;`,
		jsonStr(mapID)))
	// Remove existing marker with same ID
	b.WriteString(fmt.Sprintf(
		`if(map._via_markers[%[1]s]){map._via_markers[%[1]s].remove();delete map._via_markers[%[1]s];}`,
		jsonStr(markerID)))
	b.WriteString(markerBodyJS(markerID, mk))
	b.WriteString(`})()`)
	return b.String()
}

// removeMarkerJS generates JS to remove a marker. Expects `m` in scope (used inside guard).
func removeMarkerJS(markerID string) string {
	return fmt.Sprintf(
		`if(m._via_markers[%[1]s]){m._via_markers[%[1]s].remove();delete m._via_markers[%[1]s];}`,
		jsonStr(markerID))
}

// popupBodyJS generates JS to show a popup, assuming `map` is in scope.
func popupBodyJS(popupID string, p Popup) string {
	var b strings.Builder
	b.WriteString(popupConstructorJS(p, "p"))
	b.WriteString(fmt.Sprintf(
		`p.setLngLat([%s,%s]).addTo(map);map._via_popups[%s]=p;`,
		formatFloat(p.LngLat.Lng), formatFloat(p.LngLat.Lat), jsonStr(popupID)))
	return b.String()
}

// showPopupJS generates a self-contained IIFE to show a popup post-render.
func showPopupJS(mapID, popupID string, p Popup) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		`(function(){var map=window.__via_maps&&window.__via_maps[%s];if(!map)return;`,
		jsonStr(mapID)))
	// Close existing popup with same ID
	b.WriteString(fmt.Sprintf(
		`if(map._via_popups[%[1]s]){map._via_popups[%[1]s].remove();delete map._via_popups[%[1]s];}`,
		jsonStr(popupID)))
	b.WriteString(popupBodyJS(popupID, p))
	b.WriteString(`})()`)
	return b.String()
}

// closePopupJS generates JS to close a popup. Expects `m` in scope (used inside guard).
func closePopupJS(popupID string) string {
	return fmt.Sprintf(
		`if(m._via_popups[%[1]s]){m._via_popups[%[1]s].remove();delete m._via_popups[%[1]s];}`,
		jsonStr(popupID))
}

// popupConstructorJS generates JS to create a Popup object stored in varName.
func popupConstructorJS(p Popup, varName string) string {
	opts := "{"
	if p.HideCloseButton {
		opts += `closeButton:false,`
	}
	if p.MaxWidth != "" {
		opts += fmt.Sprintf(`maxWidth:%s,`, jsonStr(p.MaxWidth))
	}
	opts += "}"
	return fmt.Sprintf(`var %s=new maplibregl.Popup(%s).setHTML(%s);`,
		varName, opts, jsonStr(p.Content))
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
