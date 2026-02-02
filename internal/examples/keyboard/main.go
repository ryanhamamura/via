package main

import (
	"fmt"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

const gridSize = 8

func main() {
	v := via.New()
	v.Config(via.Options{DocumentTitle: "Keyboard", ServerAddress: ":7331"})

	v.Page("/", func(c *via.Context) {
		x, y := 0, 0
		dir := c.Signal("")

		move := c.Action(func() {
			switch dir.String() {
			case "up":
				y = max(0, y-1)
			case "down":
				y = min(gridSize-1, y+1)
			case "left":
				x = max(0, x-1)
			case "right":
				x = min(gridSize-1, x+1)
			}
			c.Sync()
		})

		c.View(func() h.H {
			var rows []h.H
			for row := range gridSize {
				var cells []h.H
				for col := range gridSize {
					bg := "#e0e0e0"
					if col == x && row == y {
						bg = "#4a90d9"
					}
					cells = append(cells, h.Div(
						h.Attr("style", fmt.Sprintf(
							"width:48px;height:48px;background:%s;border:1px solid #ccc;",
							bg,
						)),
					))
				}
				rows = append(rows, h.Div(
					append([]h.H{h.Attr("style", "display:flex;")}, cells...)...,
				))
			}

			return h.Div(
				h.H1(h.Text("Keyboard Grid")),
				h.P(h.Text("Move with WASD or arrow keys")),
				h.Div(rows...),
				via.OnKeyDownMap(
					via.KeyBind("w", move, via.WithSignal(dir, "up")),
					via.KeyBind("a", move, via.WithSignal(dir, "left")),
					via.KeyBind("s", move, via.WithSignal(dir, "down")),
					via.KeyBind("d", move, via.WithSignal(dir, "right")),
					via.KeyBind("ArrowUp", move, via.WithSignal(dir, "up"), via.WithPreventDefault()),
					via.KeyBind("ArrowLeft", move, via.WithSignal(dir, "left"), via.WithPreventDefault()),
					via.KeyBind("ArrowDown", move, via.WithSignal(dir, "down"), via.WithPreventDefault()),
					via.KeyBind("ArrowRight", move, via.WithSignal(dir, "right"), via.WithPreventDefault()),
				),
			)
		})
	})

	v.Start()
}
