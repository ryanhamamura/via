// Spike to validate that Datastar's data-effect re-evaluates when signals are
// updated via PatchSignals from the server, and that Via's hex signal IDs work
// in $signalID expression syntax.
package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

func main() {
	v := via.New()
	v.Config(via.Options{
		DocumentTitle: "data-effect Spike",
		ServerAddress: ":7332",
		DevMode:       true,
	})

	v.Page("/", func(c *via.Context) {
		x := c.Signal(0)
		y := c.Signal(0)

		c.OnInterval(time.Second, func() {
			x.SetValue(rand.Intn(500))
			y.SetValue(rand.Intn(500))
			c.SyncSignals()
		})

		c.View(func() h.H {
			return h.Div(
				h.Attr("style", "padding:1rem;font-family:sans-serif"),
				h.H1(h.Text("data-effect Spike")),
				h.P(h.Text("x: "), x.Text(), h.Text(" y: "), y.Text()),
				h.Div(
					h.ID("box"),
					h.Attr("style", "width:20px;height:20px;background:red;position:absolute"),
					h.DataEffect(fmt.Sprintf(
						"document.getElementById('box').style.left=$%s+'px';"+
							"document.getElementById('box').style.top=$%s+'px'",
						x.ID(), y.ID(),
					)),
				),
			)
		})
	})

	v.Start()
}
