package main

import (
	"sync"
	"time"

	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

var (
	WithSignal = via.WithSignal
)

func ChatPage(c *via.Context) {
	currentUser := UserInfo{
		Name:  c.Session().GetString(SessionKeyUsername),
		Emoji: c.Session().GetString(SessionKeyEmoji),
	}

	roomSignal := c.Signal("Go")
	statement := c.Signal("")

	var messages []ChatMessage
	var messagesMu sync.Mutex
	currentRoom := "Go"

	var currentSub via.Subscription

	subscribeToRoom := func(room string) {
		if currentSub != nil {
			currentSub.Unsubscribe()
		}

		subject := "chat.room." + room

		if hist, err := via.ReplayHistory[ChatMessage](v, subject, 50); err == nil {
			messages = hist
		}

		sub, _ := via.Subscribe(c, subject, func(msg ChatMessage) {
			messagesMu.Lock()
			messages = append(messages, msg)
			if len(messages) > 50 {
				messages = messages[len(messages)-50:]
			}
			messagesMu.Unlock()
			c.Sync()
		})
		currentSub = sub
		currentRoom = room
	}

	subscribeToRoom("Go")

	// Heartbeat — keeps connected indicator alive
	connected := true
	c.OnInterval(30*time.Second, func() {
		connected = true
		c.Sync()
	})

	switchRoom := c.Action(func() {
		newRoom := roomSignal.String()
		if newRoom != currentRoom {
			messagesMu.Lock()
			messages = nil
			messagesMu.Unlock()
			subscribeToRoom(newRoom)
			c.Sync()
		}
	})

	say := c.Action(func() {
		msg := statement.String()
		if msg == "" {
			msg = randomDevQuote()
		}
		statement.SetValue("")

		via.Publish(c, "chat.room."+currentRoom, ChatMessage{
			User:    currentUser,
			Message: msg,
			Time:    time.Now().UnixMilli(),
		})
	})

	c.View(func() h.H {
		var tabs []h.H
		for _, name := range roomNames {
			isCurrent := name == currentRoom
			tabs = append(tabs, h.Li(
				h.A(
					h.If(isCurrent, h.Attr("aria-current", "page")),
					h.Text(name),
					switchRoom.OnClick(WithSignal(roomSignal, name)),
				),
			))
		}

		messagesMu.Lock()
		chatHistoryChildren := []h.H{
			h.Class("chat-history"),
			h.Script(h.Raw(`new MutationObserver(()=>scrollChatToBottom()).observe(document.querySelector('.chat-history'), {childList:true})`)),
		}
		for _, msg := range messages {
			chatHistoryChildren = append(chatHistoryChildren,
				h.Div(h.Class("chat-message"),
					h.Div(h.Class("avatar"), h.Attr("title", msg.User.Name), h.Text(msg.User.Emoji)),
					h.Div(h.Class("bubble"),
						h.P(h.Text(msg.Message)),
					),
				),
			)
		}
		messagesMu.Unlock()

		_ = connected

		return h.Div(h.Class("chat-page"),
			h.Nav(
				h.Attr("role", "tab-control"),
				h.Ul(tabs...),
				h.Span(h.Class("nats-badge"),
					h.Span(h.Class("status-dot")),
					h.Text("NATS"),
				),
			),
			h.Div(chatHistoryChildren...),
			h.Div(
				h.Class("chat-input"),
				h.DataIgnoreMorph(),
				currentUser.Avatar(),
				h.FieldSet(
					h.Attr("role", "group"),
					h.Input(
						h.Type("text"),
						h.Placeholder(currentUser.Name+" says..."),
						statement.Bind(),
						h.Attr("autofocus"),
						say.OnKeyDown("Enter"),
					),
					h.Button(h.Text("Send"), say.OnClick()),
				),
			),
		)
	})
}
