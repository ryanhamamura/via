package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
	"github.com/ryanhamamura/via/vianats"
)

var (
	WithSignal = via.WithSignal
)

// ChatMessage represents a message in a chat room
type ChatMessage struct {
	User    UserInfo `json:"user"`
	Message string   `json:"message"`
	Time    int64    `json:"time"`
}

// UserInfo identifies a chat participant
type UserInfo struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

func (u *UserInfo) Avatar() h.H {
	return h.Div(h.Class("avatar"), h.Attr("title", u.Name), h.Text(u.Emoji))
}

var roomNames = []string{"Go", "Rust", "Python", "JavaScript", "Clojure"}

func main() {
	ctx := context.Background()

	ps, err := vianats.New(ctx, "./data/nats")
	if err != nil {
		log.Fatalf("Failed to start embedded NATS: %v", err)
	}
	defer ps.Close()

	// Create JetStream stream for message durability
	js := ps.JetStream()
	js.AddStream(&nats.StreamConfig{
		Name:      "CHAT",
		Subjects:  []string{"chat.>"},
		Retention: nats.LimitsPolicy,
		MaxMsgs:   1000,
		MaxAge:    24 * time.Hour,
	})

	v := via.New()
	v.Config(via.Options{
		DevMode:       true,
		DocumentTitle: "NATS Chat",
		LogLvl:        via.LogLevelInfo,
		ServerAddress: ":7331",
		PubSub:        ps,
	})

	v.AppendToHead(
		h.Link(h.Rel("stylesheet"), h.Href("https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css")),
		h.StyleEl(h.Raw(`
			body { margin: 0; }
			main {
				display: flex;
				flex-direction: column;
				height: 100vh;
			}
			nav[role="tab-control"] ul li a[aria-current="page"] {
				background-color: var(--pico-primary-background);
				color: var(--pico-primary-inverse);
				border-bottom: 2px solid var(--pico-primary);
			}
			.chat-message { display: flex; gap: 0.75rem; margin-bottom: 0.5rem; }
			.avatar {
				width: 2rem;
				height: 2rem;
				border-radius: 50%;
				background: var(--pico-muted-border-color);
				display: grid;
				place-items: center;
				font-size: 1.5rem;
				flex-shrink: 0;
			}
			.bubble { flex: 1; }
			.bubble p { margin: 0; }
			.chat-history {
				flex: 1;
				overflow-y: auto;
				padding: 1rem;
				padding-bottom: calc(88px + env(safe-area-inset-bottom));
			}
			.chat-input {
				position: fixed;
				left: 0;
				right: 0;
				bottom: 0;
				background: var(--pico-background-color);
				display: flex;
				align-items: center;
				gap: 0.75rem;
				padding: 0.75rem 1rem calc(0.75rem + env(safe-area-inset-bottom));
				border-top: 1px solid var(--pico-muted-border-color);
			}
			.chat-input fieldset {
				flex: 1;
				margin: 0;
			}
			.nats-badge {
				background: #27AAE1;
				color: white;
				padding: 0.25rem 0.5rem;
				border-radius: 4px;
				font-size: 0.75rem;
				margin-left: auto;
			}
		`)),
		h.Script(h.Raw(`
			function scrollChatToBottom() {
				const chatHistory = document.querySelector('.chat-history');
				if (chatHistory) chatHistory.scrollTop = chatHistory.scrollHeight;
			}
		`)),
	)

	v.Page("/", func(c *via.Context) {
		currentUser := randUser()
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
			sub, _ := c.Subscribe("chat.room."+room, func(data []byte) {
				var msg ChatMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					return
				}
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

			data, _ := json.Marshal(ChatMessage{
				User:    currentUser,
				Message: msg,
				Time:    time.Now().UnixMilli(),
			})
			c.Publish("chat.room."+currentRoom, data)
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

			return h.Main(h.Class("container"),
				h.Nav(
					h.Attr("role", "tab-control"),
					h.Ul(tabs...),
					h.Span(h.Class("nats-badge"), h.Text("NATS")),
				),
				h.Div(chatHistoryChildren...),
				h.Div(
					h.Class("chat-input"),
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
	})

	log.Println("Starting NATS chatroom on :7331 (embedded NATS server)")
	v.Start()
}

func randUser() UserInfo {
	adjectives := []string{"Happy", "Clever", "Brave", "Swift", "Gentle", "Wise", "Bold", "Calm", "Eager", "Fierce"}
	animals := []string{"Panda", "Tiger", "Eagle", "Dolphin", "Fox", "Wolf", "Bear", "Hawk", "Otter", "Lion"}
	emojis := []string{"🐼", "🐯", "🦅", "🐬", "🦊", "🐺", "🐻", "🦅", "🦦", "🦁"}

	idx := rand.Intn(len(animals))
	return UserInfo{
		Name:  adjectives[rand.Intn(len(adjectives))] + " " + animals[idx],
		Emoji: emojis[idx],
	}
}

var quoteIdx = rand.Intn(len(devQuotes))
var devQuotes = []string{
	"Just use NATS.",
	"Pub/sub all the things!",
	"Messages are the new API.",
	"JetStream for durability.",
	"No more polling.",
	"Event-driven architecture FTW.",
	"Decouple everything.",
	"NATS is fast.",
	"Subjects are like topics.",
	"Request-reply is cool.",
}

func randomDevQuote() string {
	quoteIdx = (quoteIdx + 1) % len(devQuotes)
	return devQuotes[quoteIdx]
}
