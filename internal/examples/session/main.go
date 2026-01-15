package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

func main() {
	// Open SQLite database for persistent sessions
	db, err := sql.Open("sqlite3", "sessions.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create session manager with SQLite store
	sm, err := via.NewSQLiteSessionManager(db)
	if err != nil {
		log.Fatalf("failed to create session manager: %v", err)
	}

	v := via.New()
	v.Config(via.Options{
		ServerAddress:  ":7331",
		SessionManager: sm,
	})

	// Login page
	v.Page("/login", func(c *via.Context) {
		flash := c.Session().PopString("flash")
		usernameInput := c.Signal("")

		login := c.Action(func() {
			name := usernameInput.String()
			if name != "" {
				c.Session().Set("username", name)
				c.Session().Set("flash", "Welcome, "+name+"!")
				c.Session().RenewToken()
				c.Redirect("/dashboard")
			}
		})

		c.View(func() h.H {
			// Already logged in? Redirect to dashboard
			if c.Session().GetString("username") != "" {
				c.Redirect("/dashboard")
				return h.Div()
			}

			var flashMsg h.H
			if flash != "" {
				flashMsg = h.P(h.Text(flash), h.Style("color: green"))
			}
			return h.Div(
				flashMsg,
				h.H1(h.Text("Login")),
				h.Input(h.Type("text"), h.Placeholder("Username"), usernameInput.Bind()),
				h.Button(h.Text("Login"), login.OnClick()),
			)
		})
	})

	// Dashboard page (protected)
	v.Page("/dashboard", func(c *via.Context) {
		logout := c.Action(func() {
			c.Session().Set("flash", "Goodbye!")
			c.Session().Delete("username")
			c.Redirect("/login")
		})

		c.View(func() h.H {
			username := c.Session().GetString("username")

			// Not logged in? Redirect to login
			if username == "" {
				c.Session().Set("flash", "Please log in first")
				c.Redirect("/login")
				return h.Div()
			}

			flash := c.Session().PopString("flash")
			var flashMsg h.H
			if flash != "" {
				flashMsg = h.P(h.Text(flash), h.Style("color: green"))
			}
			return h.Div(
				flashMsg,
				h.H1(h.Textf("Dashboard - Hello, %s!", username)),
				h.P(h.Text("Your session persists across page refreshes.")),
				h.Button(h.Text("Logout"), logout.OnClick()),
			)
		})
	})

	// Redirect root to login
	v.Page("/", func(c *via.Context) {
		c.View(func() h.H {
			c.Redirect("/login")
			return h.Div()
		})
	})

	v.Start()
}
