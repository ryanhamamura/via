package main

import (
	_ "embed"
	"log"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

//go:embed chat.css
var chatCSS string

var v *via.V

func main() {
	v = via.New()
	v.Config(via.Options{
		DevMode:       true,
		DocumentTitle: "NATS Chat",
		LogLevel:      via.LogLevelInfo,
		ServerAddress: ":7331",
	})

	err := via.EnsureStream(v, via.StreamConfig{
		Name:     "CHAT",
		Subjects: []string{"chat.>"},
		MaxMsgs:  1000,
		MaxAge:   24 * time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to ensure stream: %v", err)
	}

	v.AppendToHead(
		h.Link(h.Rel("stylesheet"), h.Href("https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css")),
		h.StyleEl(h.Raw(chatCSS)),
		h.Script(h.Raw(`
			function scrollChatToBottom() {
				const chatHistory = document.querySelector('.chat-history');
				if (chatHistory) chatHistory.scrollTop = chatHistory.scrollHeight;
			}
		`)),
	)

	v.Layout(func(content func() h.H) h.H {
		return h.Div(
			h.Nav(h.Class("app-nav"),
				h.A(h.Href("/"), h.Class("brand"), h.Text("NATS Chat")),
				h.Div(h.Class("nav-links"),
					h.A(h.Href("/"), h.Text("Chat")),
					h.A(h.Href("/profile"), h.Text("Profile")),
				),
			),
			h.Main(h.Class("container"),
				h.DataViewTransition("page-content"),
				content(),
			),
		)
	})

	// Profile page — public, no auth required
	v.Page("/profile", ProfilePage)

	// Auth middleware — redirects to profile if no identity set
	requireProfile := func(c *via.Context, next func()) {
		if c.Session().GetString(SessionKeyUsername) == "" {
			c.RedirectView("/profile")
			return
		}
		next()
	}

	// Chat page — protected by profile middleware
	protected := v.Group("", requireProfile)
	protected.Page("/", ChatPage)

	log.Println("Starting NATS chatroom on :7331 (embedded NATS server)")
	v.Start()
}
