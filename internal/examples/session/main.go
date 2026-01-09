package main

import (
	"github.com/go-via/via"
	"github.com/go-via/via/h"
)

func main() {
	v := via.New()

	v.Page("/", func(c *via.Context) {
		username := c.Session().GetString("username")
		flash := c.Session().PopString("flash")

		usernameInput := c.Signal("")

		login := c.Action(func() {
			name := usernameInput.String()
			if name != "" {
				c.Session().Set("username", name)
				c.Session().Set("flash", "Welcome, "+name+"!")
				c.Session().RenewToken()
			}
			c.Sync()
		})

		logout := c.Action(func() {
			c.Session().Set("flash", "Goodbye!")
			c.Session().Delete("username")
			c.Sync()
		})

		c.View(func() h.H {
			var flashMsg h.H
			if flash != "" {
				flashMsg = h.P(h.Text(flash), h.Style("color: green"))
			}

			if username == "" {
				return h.Div(
					flashMsg,
					h.H1(h.Text("Login")),
					h.Input(h.Type("text"), h.Placeholder("Username"), usernameInput.Bind()),
					h.Button(h.Text("Login"), login.OnClick()),
				)
			}
			return h.Div(
				flashMsg,
				h.H1(h.Textf("Hello, %s!", username)),
				h.P(h.Text("Your session persists across page refreshes.")),
				h.Button(h.Text("Logout"), logout.OnClick()),
			)
		})
	})

	v.Start()
}
