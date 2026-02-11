package main

import (
	"fmt"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

func main() {
	v := via.New()
	v.Config(via.Options{
		ServerAddress: ":8080",
		DocumentTitle: "Middleware Example",
	})

	// --- Middleware definitions ---

	// requestLogger logs every page request to stdout.
	requestLogger := func(c *via.Context, next func()) {
		fmt.Printf("[%s] request\n", time.Now().Format("15:04:05"))
		next()
	}

	// authRequired redirects unauthenticated users to /login.
	authRequired := func(c *via.Context, next func()) {
		if c.Session().GetString("role") == "" {
			c.RedirectView("/login")
			return
		}
		next()
	}

	// auditLog prints the authenticated username to stdout.
	auditLog := func(c *via.Context, next func()) {
		fmt.Printf("[audit] user=%s\n", c.Session().GetString("username"))
		next()
	}

	// superAdminOnly rejects non-superadmin users with a forbidden view.
	superAdminOnly := func(c *via.Context, next func()) {
		if c.Session().GetString("role") != "superadmin" {
			c.View(func() h.H {
				return h.Div(
					h.H1(h.Text("Forbidden")),
					h.P(h.Text("Super-admin access required.")),
					h.A(h.Href("/admin/dashboard"), h.Text("Back to dashboard")),
				)
			})
			return
		}
		next()
	}

	// --- Route registration ---

	v.Use(requestLogger) // global middleware

	admin := v.Group("/admin", authRequired)            // prefixed group
	admin.Use(auditLog)                                 // Group.Use()
	superAdmin := admin.Group("/super", superAdminOnly) // nested group

	// Public: redirect root to login
	v.Page("/", func(c *via.Context) {
		c.View(func() h.H {
			c.Redirect("/login")
			return h.Div()
		})
	})

	// Public: login page with role-selection buttons
	v.Page("/login", func(c *via.Context) {
		loginAdmin := c.Action(func() {
			c.Session().Set("role", "admin")
			c.Session().Set("username", "alice")
			c.Session().RenewToken()
			c.Redirect("/admin/dashboard")
		})

		loginSuper := c.Action(func() {
			c.Session().Set("role", "superadmin")
			c.Session().Set("username", "bob")
			c.Session().RenewToken()
			c.Redirect("/admin/dashboard")
		})

		c.View(func() h.H {
			return h.Div(
				h.H1(h.Text("Login")),
				h.P(h.Text("Choose a role:")),
				h.Button(h.Text("Login as Admin"), loginAdmin.OnClick()),
				h.Raw(" "),
				h.Button(h.Text("Login as Super Admin"), loginSuper.OnClick()),
			)
		})
	})

	// Admin: dashboard (requires authRequired + auditLog)
	admin.Page("/dashboard", func(c *via.Context) {
		logout := c.Action(func() {
			c.Session().Delete("role")
			c.Session().Delete("username")
			c.Redirect("/login")
		})

		c.View(func() h.H {
			username := c.Session().GetString("username")
			role := c.Session().GetString("role")
			return h.Div(
				h.H1(h.Textf("Dashboard — %s (%s)", username, role)),
				h.Ul(
					h.Li(h.A(h.Href("/admin/super/settings"), h.Text("Super Admin Settings"))),
				),
				h.Button(h.Text("Logout"), logout.OnClick()),
			)
		})
	})

	// Super-admin: settings (requires authRequired + auditLog + superAdminOnly)
	superAdmin.Page("/settings", func(c *via.Context) {
		c.View(func() h.H {
			username := c.Session().GetString("username")
			return h.Div(
				h.H1(h.Textf("Super Admin Settings — %s", username)),
				h.P(h.Text("Only super-admins can see this page.")),
				h.A(h.Href("/admin/dashboard"), h.Text("Back to dashboard")),
			)
		})
	})

	v.Start()
}
