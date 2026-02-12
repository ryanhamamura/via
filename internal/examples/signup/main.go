package main

import (
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

func main() {
	v := via.New()
	v.Config(via.Options{
		DocumentTitle: "Signup",
		ServerAddress: ":8080",
	})

	v.AppendToHead(h.StyleEl(h.Raw(`
		body { font-family: system-ui, sans-serif; max-width: 420px; margin: 2rem auto; padding: 0 1rem; }
		label { display: block; font-weight: 600; margin-top: 1rem; }
		input { display: block; width: 100%; padding: 0.4rem; margin-top: 0.25rem; box-sizing: border-box; }
		.error { color: #c00; font-size: 0.85rem; margin-top: 0.2rem; }
		.success { color: #080; margin-top: 1rem; }
		.actions { margin-top: 1.5rem; display: flex; gap: 0.5rem; }
	`)))

	v.Page("/", func(c *via.Context) {
		username := c.Field("", via.Required(), via.MinLen(3), via.MaxLen(20))
		email := c.Field("", via.Required(), via.Email())
		age := c.Field("", via.Required(), via.Min(13), via.Max(120))
		// Optional field — only validated when non-empty
		website := c.Field("", via.Custom(func(val string) error { return nil }), via.Pattern(`^$|^https?://\S+$`, "Must be a valid URL"))

		var success string

		signup := c.Action(func() {
			success = ""
			if !c.ValidateAll(username, email, age, website) {
				c.Sync()
				return
			}
			// Server-side check
			if username.String() == "admin" {
				username.AddError("Username is already taken")
				c.Sync()
				return
			}
			success = "Account created for " + username.String() + "!"
			c.ResetFields(username, email, age, website)
			c.Sync()
		})

		reset := c.Action(func() {
			success = ""
			c.ResetFields(username, email, age, website)
			c.Sync()
		})

		c.View(func() h.H {
			return h.Div(
				h.H1(h.Text("Sign Up")),

				h.Label(h.Text("Username")),
				h.Input(h.Type("text"), h.Placeholder("pick a username"), username.Bind()),
				h.If(username.HasError(), h.Div(h.Class("error"), h.Text(username.FirstError()))),

				h.Label(h.Text("Email")),
				h.Input(h.Type("email"), h.Placeholder("you@example.com"), email.Bind()),
				h.If(email.HasError(), h.Div(h.Class("error"), h.Text(email.FirstError()))),

				h.Label(h.Text("Age")),
				h.Input(h.Type("number"), h.Placeholder("your age"), age.Bind()),
				h.If(age.HasError(), h.Div(h.Class("error"), h.Text(age.FirstError()))),

				h.Label(h.Text("Website (optional)")),
				h.Input(h.Type("url"), h.Placeholder("https://example.com"), website.Bind()),
				h.If(website.HasError(), h.Div(h.Class("error"), h.Text(website.FirstError()))),

				h.Div(h.Class("actions"),
					h.Button(h.Text("Sign Up"), signup.OnClick()),
					h.Button(h.Text("Reset"), reset.OnClick()),
				),

				h.If(success != "", h.P(h.Class("success"), h.Text(success))),
			)
		})
	})

	v.Start()
}
