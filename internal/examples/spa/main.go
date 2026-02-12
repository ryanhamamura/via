package main

import (
	"fmt"
	"time"

	"github.com/ryanhamamura/via"
	. "github.com/ryanhamamura/via/h"
)

func main() {
	v := via.New()
	v.Config(via.Options{
		DocumentTitle: "SPA Navigation",
		ServerAddress: ":7331",
	})

	v.AppendToHead(
		Raw(`<link rel="preconnect" href="https://fonts.googleapis.com">`),
		Raw(`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>`),
		Raw(`<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700&display=swap" rel="stylesheet">`),
		Raw(`<style>body{font-family:'Inter',sans-serif;margin:0;background:#111;color:#eee}</style>`),
	)

	v.Layout(func(content func() H) H {
		return Div(
			Nav(
				Style("display:flex;gap:1rem;padding:1rem;background:#222;"),
				A(Href("/"), Text("Home"), Style("color:#fff")),
				A(Href("/counter"), Text("Counter"), Style("color:#fff")),
				A(Href("/clock"), Text("Clock"), Style("color:#fff")),
				A(Href("https://github.com"), Text("GitHub (external)"), Style("color:#888")),
				A(Href("/"), Text("Full Reload"), Attr("data-via-no-boost"), Style("color:#f88")),
			),
			Main(Style("padding:1rem"), content()),
		)
	})

	// Home page
	v.Page("/", func(c *via.Context) {
		goCounter := c.Action(func() { c.Navigate("/counter", false) })

		c.View(func() H {
			return Div(
				H1(Text("Home"), DataViewTransition("page-title")),
				P(Text("Click the nav links above — no page reload, no white flash.")),
				P(Text("Or navigate programmatically:")),
				Button(Text("Go to Counter"), goCounter.OnClick()),
			)
		})
	})

	// Counter page — demonstrates signals and actions survive within a page,
	// but reset on navigate away and back.
	v.Page("/counter", func(c *via.Context) {
		count := 0
		increment := c.Action(func() { count++; c.Sync() })
		goHome := c.Action(func() { c.Navigate("/", false) })

		c.View(func() H {
			return Div(
				H1(Text("Counter"), DataViewTransition("page-title")),
				P(Textf("Count: %d", count)),
				Button(Text("+1"), increment.OnClick()),
				Button(Text("Go Home"), goHome.OnClick(), Style("margin-left:0.5rem")),
			)
		})
	})

	// Clock page — demonstrates OnInterval cleanup on navigate.
	v.Page("/clock", func(c *via.Context) {
		now := time.Now().Format("15:04:05")
		c.OnInterval(time.Second, func() {
			now = time.Now().Format("15:04:05")
			c.Sync()
		})

		c.View(func() H {
			return Div(
				H1(Text("Clock"), DataViewTransition("page-title")),
				P(Text("This page has an OnInterval that ticks every second.")),
				P(Textf("Current time: %s", now)),
				P(Text("Navigate away and back — the old interval stops, a new one starts.")),
				P(Textf("Proof this is a fresh page init: random = %d", time.Now().UnixNano()%1000)),
			)
		})
	})

	fmt.Println("SPA example running at http://localhost:7331")
	v.Start()
}
