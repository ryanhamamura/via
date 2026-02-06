package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"log"
	"sync"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
	"github.com/ryanhamamura/via/vianats"
)

var WithSignal = via.WithSignal

type Bookmark struct {
	ID    string
	Title string
	URL   string
}

type CRUDEvent struct {
	Action string `json:"action"`
	Title  string `json:"title"`
	UserID string `json:"user_id"`
}

var (
	bookmarks   []Bookmark
	bookmarksMu sync.RWMutex
)

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func findBookmark(id string) (Bookmark, int) {
	for i, bm := range bookmarks {
		if bm.ID == id {
			return bm, i
		}
	}
	return Bookmark{}, -1
}

func main() {
	ctx := context.Background()

	ps, err := vianats.New(ctx, "./data/nats")
	if err != nil {
		log.Fatalf("Failed to start embedded NATS: %v", err)
	}
	defer ps.Close()

	err = vianats.EnsureStream(ps, vianats.StreamConfig{
		Name:     "BOOKMARKS",
		Subjects: []string{"bookmarks.>"},
		MaxMsgs:  1000,
		MaxAge:   24 * time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to ensure stream: %v", err)
	}

	v := via.New()
	v.Config(via.Options{
		DevMode:       true,
		DocumentTitle: "Bookmarks",
		LogLevel:      via.LogLevelInfo,
		ServerAddress: ":7331",
		PubSub:        ps,
	})

	v.AppendToHead(
		h.Link(h.Rel("stylesheet"), h.Href("https://cdn.jsdelivr.net/npm/daisyui@4/dist/full.min.css")),
		h.Script(h.Src("https://cdn.tailwindcss.com")),
	)

	v.Page("/", func(c *via.Context) {
		userID := randomHex(8)

		titleSignal := c.Signal("")
		urlSignal := c.Signal("")
		targetIDSignal := c.Signal("")

		via.Subscribe(c, "bookmarks.events", func(evt CRUDEvent) {
			if evt.UserID == userID {
				return
			}
			safeTitle := html.EscapeString(evt.Title)
			var alertClass string
			switch evt.Action {
			case "created":
				alertClass = "alert-success"
			case "updated":
				alertClass = "alert-info"
			case "deleted":
				alertClass = "alert-error"
			}
			c.ExecScript(fmt.Sprintf(`(function(){
				var tc = document.getElementById('toast-container');
				if (!tc) return;
				var d = document.createElement('div');
				d.className = 'alert %s';
				d.innerHTML = '<span>Bookmark "%s" %s</span>';
				tc.appendChild(d);
				setTimeout(function(){ d.remove(); }, 3000);
			})()`, alertClass, safeTitle, evt.Action))
			c.Sync()
		})

		save := c.Action(func() {
			title := titleSignal.String()
			url := urlSignal.String()
			if title == "" || url == "" {
				return
			}

			targetID := targetIDSignal.String()
			action := "created"

			bookmarksMu.Lock()
			if targetID != "" {
				if _, idx := findBookmark(targetID); idx >= 0 {
					bookmarks[idx].Title = title
					bookmarks[idx].URL = url
					action = "updated"
				}
			} else {
				bookmarks = append(bookmarks, Bookmark{
					ID:    randomHex(8),
					Title: title,
					URL:   url,
				})
			}
			bookmarksMu.Unlock()

			titleSignal.SetValue("")
			urlSignal.SetValue("")
			targetIDSignal.SetValue("")

			via.Publish(c, "bookmarks.events", CRUDEvent{
				Action: action,
				Title:  title,
				UserID: userID,
			})
			c.Sync()
		})

		edit := c.Action(func() {
			id := targetIDSignal.String()
			bookmarksMu.RLock()
			bm, idx := findBookmark(id)
			bookmarksMu.RUnlock()
			if idx < 0 {
				return
			}
			titleSignal.SetValue(bm.Title)
			urlSignal.SetValue(bm.URL)
		})

		del := c.Action(func() {
			id := targetIDSignal.String()
			bookmarksMu.Lock()
			bm, idx := findBookmark(id)
			if idx >= 0 {
				bookmarks = append(bookmarks[:idx], bookmarks[idx+1:]...)
			}
			bookmarksMu.Unlock()
			if idx < 0 {
				return
			}

			targetIDSignal.SetValue("")

			via.Publish(c, "bookmarks.events", CRUDEvent{
				Action: "deleted",
				Title:  bm.Title,
				UserID: userID,
			})
			c.Sync()
		})

		cancelEdit := c.Action(func() {
			titleSignal.SetValue("")
			urlSignal.SetValue("")
			targetIDSignal.SetValue("")
		})

		c.View(func() h.H {
			isEditing := targetIDSignal.String() != ""

			// Build table rows
			bookmarksMu.RLock()
			var rows []h.H
			for _, bm := range bookmarks {
				rows = append(rows, h.Tr(
					h.Td(h.Text(bm.Title)),
					h.Td(h.A(h.Href(bm.URL), h.Attr("target", "_blank"), h.Class("link link-primary"), h.Text(bm.URL))),
					h.Td(
						h.Div(h.Class("flex gap-1"),
							h.Button(h.Class("btn btn-xs btn-ghost"), h.Text("Edit"),
								edit.OnClick(WithSignal(targetIDSignal, bm.ID)),
							),
							h.Button(h.Class("btn btn-xs btn-ghost text-error"), h.Text("Delete"),
								del.OnClick(WithSignal(targetIDSignal, bm.ID)),
							),
						),
					),
				))
			}
			bookmarksMu.RUnlock()

			saveLabel := "Add Bookmark"
			if isEditing {
				saveLabel = "Update Bookmark"
			}

			return h.Div(h.Class("min-h-screen bg-base-200"),
				// Navbar
				h.Div(h.Class("navbar bg-base-100 shadow-sm"),
					h.Div(h.Class("flex-1"),
						h.A(h.Class("btn btn-ghost text-xl"), h.Text("Bookmarks")),
					),
					h.Div(h.Class("flex-none"),
						h.Div(h.Class("badge badge-outline"), h.Text(userID[:8])),
					),
				),

				h.Div(h.Class("container mx-auto p-4 max-w-3xl flex flex-col gap-4"),
					// Form card
					h.Div(h.Class("card bg-base-100 shadow"),
						h.Div(h.Class("card-body"),
							h.H2(h.Class("card-title"), h.Text(saveLabel)),
							h.Div(h.Class("flex flex-col gap-2"),
								h.Input(h.Class("input input-bordered w-full"), h.Type("text"), h.Placeholder("Title"), titleSignal.Bind()),
								h.Input(h.Class("input input-bordered w-full"), h.Type("text"), h.Placeholder("https://example.com"), urlSignal.Bind()),
								h.Div(h.Class("card-actions justify-end"),
									h.If(isEditing,
										h.Button(h.Class("btn btn-ghost"), h.Text("Cancel"), cancelEdit.OnClick()),
									),
									h.Button(h.Class("btn btn-primary"), h.Text(saveLabel), save.OnClick()),
								),
							),
						),
					),

					// Table card
					h.Div(h.Class("card bg-base-100 shadow"),
						h.Div(h.Class("card-body"),
							h.H2(h.Class("card-title"), h.Text("All Bookmarks")),
							h.If(len(rows) == 0,
								h.P(h.Class("text-base-content/60"), h.Text("No bookmarks yet. Add one above!")),
							),
							h.If(len(rows) > 0,
								h.Div(h.Class("overflow-x-auto"),
									h.Table(h.Class("table"),
										h.THead(h.Tr(
											h.Th(h.Text("Title")),
											h.Th(h.Text("URL")),
											h.Th(h.Text("Actions")),
										)),
										h.TBody(rows...),
									),
								),
							),
						),
					),
				),

				// Toast container — ignored by morph so Sync() doesn't wipe active toasts
				h.Div(h.ID("toast-container"), h.Class("toast toast-end toast-top"), h.DataIgnoreMorph()),
			)
		})
	})

	log.Println("Starting pubsub-crud example on :7331")
	v.Start()
}
