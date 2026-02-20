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

	// Build constructor options object
	b.WriteString(fmt.Sprintf(
		`var opts={container:%s,style:%s,center:[%s,%s],zoom:%s`,
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

	// Interaction toggles
	writeBoolOpt := func(name string, val *bool) {
		if val != nil {
			if *val {
				b.WriteString(fmt.Sprintf(`,%s:true`, name))
			} else {
				b.WriteString(fmt.Sprintf(`,%s:false`, name))
			}
		}
	}
	writeBoolOpt("scrollZoom", m.opts.ScrollZoom)
	writeBoolOpt("boxZoom", m.opts.BoxZoom)
	writeBoolOpt("dragRotate", m.opts.DragRotate)
	writeBoolOpt("dragPan", m.opts.DragPan)
	writeBoolOpt("keyboard", m.opts.Keyboard)
	writeBoolOpt("doubleClickZoom", m.opts.DoubleClickZoom)
	writeBoolOpt("touchZoomRotate", m.opts.TouchZoomRotate)
	writeBoolOpt("touchPitch", m.opts.TouchPitch)
	writeBoolOpt("renderWorldCopies", m.opts.RenderWorldCopies)

	if m.opts.MaxBounds != nil {
		b.WriteString(fmt.Sprintf(`,maxBounds:[[%s,%s],[%s,%s]]`,
			formatFloat(m.opts.MaxBounds.SW.Lng), formatFloat(m.opts.MaxBounds.SW.Lat),
			formatFloat(m.opts.MaxBounds.NE.Lng), formatFloat(m.opts.MaxBounds.NE.Lat)))
	}

	b.WriteString(`};`)

	// Merge Extra options
	if len(m.opts.Extra) > 0 {
		extra, _ := json.Marshal(m.opts.Extra)
		b.WriteString(fmt.Sprintf(`Object.assign(opts,%s);`, string(extra)))
	}

	b.WriteString(`var map=new maplibregl.Map(opts);`)
	b.WriteString(`if(!window.__via_maps)window.__via_maps={};`)
	b.WriteString(fmt.Sprintf(`window.__via_maps[%s]=map;`, jsonStr(m.id)))
	b.WriteString(`map._via_markers={};map._via_popups={};map._via_controls={};`)

	// Pre-render sources, layers, markers, popups, controls run on 'load'
	hasLoad := len(m.sources) > 0 || len(m.layers) > 0 || len(m.markers) > 0 || len(m.popups) > 0 || len(m.controls) > 0
	if hasLoad {
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
			b.WriteString(markerBodyJS(m.id, me.id, me.marker))
		}
		for _, pe := range m.popups {
			b.WriteString(popupBodyJS(pe.id, pe.popup))
		}
		for _, ce := range m.controls {
			b.WriteString(controlBodyJS(ce.id, ce.ctrl))
		}
		b.WriteString(`});`)
	}

	// Event listeners
	for _, ev := range m.events {
		b.WriteString(eventListenerJS(m.id, ev))
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
		`else return;`+
		`inp.dispatchEvent(new Event('input',{bubbles:true}));`+
		`});`+
		`});`,
		jsonStr("_vwrap_"+m.id),
		jsonStr(m.CenterLng.ID()),
		jsonStr(m.CenterLat.ID()),
		jsonStr(m.Zoom.ID()),
		jsonStr(m.Bearing.ID()),
		jsonStr(m.Pitch.ID()),
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
func markerBodyJS(mapID, markerID string, mk Marker) string {
	var b strings.Builder

	if mk.Element != "" {
		b.WriteString(fmt.Sprintf(
			`var _mkEl=document.createElement('div');_mkEl.innerHTML=%s;`,
			jsonStr(mk.Element)))
	}

	opts := "{"
	if mk.Element != "" {
		opts += `element:_mkEl.firstElementChild||_mkEl,`
	} else if mk.Color != "" {
		opts += fmt.Sprintf(`color:%s,`, jsonStr(mk.Color))
	}
	if mk.Anchor != "" {
		opts += fmt.Sprintf(`anchor:%s,`, jsonStr(mk.Anchor))
	}
	if mk.Rotation != 0 {
		opts += fmt.Sprintf(`rotation:%s,`, formatFloat(mk.Rotation))
	}
	if mk.Draggable {
		opts += `draggable:true,`
	}
	opts += "}"

	// Determine initial position
	if mk.LngSignal != nil && mk.LatSignal != nil {
		b.WriteString(fmt.Sprintf(`var mk=new maplibregl.Marker(%s).setLngLat([%s,%s]);`,
			opts, mk.LngSignal.String(), mk.LatSignal.String()))
	} else {
		b.WriteString(fmt.Sprintf(`var mk=new maplibregl.Marker(%s).setLngLat([%s,%s]);`,
			opts, formatFloat(mk.LngLat.Lng), formatFloat(mk.LngLat.Lat)))
	}

	if mk.Popup != nil {
		b.WriteString(popupConstructorJS(*mk.Popup, "pk"))
		b.WriteString(`mk.setPopup(pk);`)
	}
	b.WriteString(fmt.Sprintf(`mk.addTo(map);map._via_markers[%s]=mk;`, jsonStr(markerID)))

	// Dragend → signal writeback
	if mk.Draggable && mk.LngSignal != nil && mk.LatSignal != nil {
		b.WriteString(dragendHandlerJS(mapID, markerID, mk))
	}

	return b.String()
}

// dragendHandlerJS generates JS that writes marker position back to signal hidden inputs on dragend.
func dragendHandlerJS(mapID, markerID string, mk Marker) string {
	return fmt.Sprintf(
		`mk.on('dragend',function(){`+
			`var pos=mk.getLngLat();`+
			`var el=document.getElementById(%[1]s);if(!el)return;`+
			`var inputs=el.querySelectorAll('input[data-bind]');`+
			`inputs.forEach(function(inp){`+
			`var sig=inp.getAttribute('data-bind');`+
			`if(sig===%[2]s){inp.value=pos.lng;inp.dispatchEvent(new Event('input',{bubbles:true}))}`+
			`if(sig===%[3]s){inp.value=pos.lat;inp.dispatchEvent(new Event('input',{bubbles:true}))}`+
			`});`+
			`});`,
		jsonStr("_vwrap_"+mapID),
		jsonStr(mk.LngSignal.ID()),
		jsonStr(mk.LatSignal.ID()),
	)
}

// markerEffectExpr generates a data-effect expression that moves a signal-backed marker
// when its signals change.
func markerEffectExpr(mapID, markerID string, mk Marker) string {
	// Read signals before the guard so Datastar tracks them as dependencies
	// even when the map/marker hasn't loaded yet on first evaluation.
	return fmt.Sprintf(
		`var lng=$%s,lat=$%s;`+
			`var m=window.__via_maps&&window.__via_maps[%s];`+
			`if(m&&m._via_markers[%s]){`+
			`m._via_markers[%s].setLngLat([lng,lat])}`,
		mk.LngSignal.ID(), mk.LatSignal.ID(),
		jsonStr(mapID), jsonStr(markerID), jsonStr(markerID),
	)
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
	b.WriteString(markerBodyJS(mapID, markerID, mk))
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

// --- Control JS ---

// controlBodyJS generates JS to add a control, assuming `map` is in scope.
func controlBodyJS(controlID string, ctrl Control) string {
	return fmt.Sprintf(
		`var ctrl=%s;map.addControl(ctrl,%s);map._via_controls[%s]=ctrl;`,
		ctrl.controlJS(), jsonStr(ctrl.controlPosition()), jsonStr(controlID))
}

// addControlJS generates a self-contained IIFE to add a control post-render.
func addControlJS(mapID, controlID string, ctrl Control) string {
	return fmt.Sprintf(
		`(function(){var map=window.__via_maps&&window.__via_maps[%[1]s];if(!map)return;`+
			`if(map._via_controls[%[2]s]){map.removeControl(map._via_controls[%[2]s]);delete map._via_controls[%[2]s];}`+
			`var ctrl=%[3]s;map.addControl(ctrl,%[4]s);map._via_controls[%[2]s]=ctrl;`+
			`})()`,
		jsonStr(mapID), jsonStr(controlID), ctrl.controlJS(), jsonStr(ctrl.controlPosition()))
}

// removeControlJS generates JS to remove a control. Expects `m` in scope.
func removeControlJS(controlID string) string {
	return fmt.Sprintf(
		`if(m._via_controls[%[1]s]){m.removeControl(m._via_controls[%[1]s]);delete m._via_controls[%[1]s];}`,
		jsonStr(controlID))
}

// --- Event JS ---

// eventListenerJS generates JS to register a map event listener that writes
// event data to a hidden signal input.
func eventListenerJS(mapID string, ev eventEntry) string {
	var handler string
	if ev.layerID != "" {
		handler = fmt.Sprintf(
			`map.on(%[1]s,%[2]s,function(e){`+
				`var d={lngLat:{Lng:e.lngLat.lng,Lat:e.lngLat.lat},point:[e.point.x,e.point.y],layerID:%[2]s};`+
				`if(e.features)d.features=e.features.map(function(f){return JSON.parse(JSON.stringify(f))});`+
				`var el=document.getElementById(%[3]s);if(!el)return;`+
				`var inp=el.querySelector('input[data-bind=%[4]s]');`+
				`if(inp){inp.value=JSON.stringify(d);inp.dispatchEvent(new Event('input',{bubbles:true}))}`+
				`});`,
			jsonStr(ev.event), jsonStr(ev.layerID),
			jsonStr("_vwrap_"+mapID), jsonStr(ev.signal.ID()),
		)
	} else {
		handler = fmt.Sprintf(
			`map.on(%[1]s,function(e){`+
				`var d={lngLat:{Lng:e.lngLat.lng,Lat:e.lngLat.lat},point:[e.point.x,e.point.y]};`+
				`var el=document.getElementById(%[2]s);if(!el)return;`+
				`var inp=el.querySelector('input[data-bind=%[3]s]');`+
				`if(inp){inp.value=JSON.stringify(d);inp.dispatchEvent(new Event('input',{bubbles:true}))}`+
				`});`,
			jsonStr(ev.event),
			jsonStr("_vwrap_"+mapID), jsonStr(ev.signal.ID()),
		)
	}
	return handler
}

// --- Camera options JS ---

// cameraOptionsJS converts CameraOptions to a JS object literal string.
func cameraOptionsJS(opts CameraOptions) string {
	obj := map[string]any{}
	if opts.Center != nil {
		obj["center"] = []float64{opts.Center.Lng, opts.Center.Lat}
	}
	if opts.Zoom != nil {
		obj["zoom"] = *opts.Zoom
	}
	if opts.Bearing != nil {
		obj["bearing"] = *opts.Bearing
	}
	if opts.Pitch != nil {
		obj["pitch"] = *opts.Pitch
	}
	if opts.Duration != nil {
		obj["duration"] = *opts.Duration
	}
	if opts.Speed != nil {
		obj["speed"] = *opts.Speed
	}
	if opts.Curve != nil {
		obj["curve"] = *opts.Curve
	}
	if opts.Padding != nil {
		obj["padding"] = map[string]int{
			"top":    opts.Padding.Top,
			"bottom": opts.Padding.Bottom,
			"left":   opts.Padding.Left,
			"right":  opts.Padding.Right,
		}
	}
	if opts.Animate != nil {
		obj["animate"] = *opts.Animate
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
