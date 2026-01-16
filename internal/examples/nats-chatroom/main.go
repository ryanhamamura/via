package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/delaneyj/toolbelt/embeddednats"
	"github.com/nats-io/nats.go"
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
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

// NATSChatroom manages NATS connections and per-context subscriptions
type NATSChatroom struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	subs map[string]*nats.Subscription
	mu   sync.RWMutex
}

func NewNATSChatroom(nc *nats.Conn) (*NATSChatroom, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	// Create or update the CHAT stream for durability
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "CHAT",
		Subjects:  []string{"chat.>"},
		Retention: nats.LimitsPolicy,
		MaxMsgs:   1000, // Keep last 1000 messages per room
		MaxAge:    24 * time.Hour,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		// Stream might already exist, that's fine
		log.Printf("Note: stream creation returned: %v", err)
	}

	return &NATSChatroom{
		nc:   nc,
		js:   js,
		subs: make(map[string]*nats.Subscription),
	}, nil
}

// Subscribe creates a subscription for a context to a room
func (chat *NATSChatroom) Subscribe(ctxID, room string, handler func(msg *ChatMessage)) error {
	subject := "chat.room." + room

	sub, err := chat.nc.Subscribe(subject, func(m *nats.Msg) {
		var msg ChatMessage
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return
		}
		handler(&msg)
	})
	if err != nil {
		return err
	}

	chat.mu.Lock()
	// Clean up old subscription if exists
	if old, exists := chat.subs[ctxID]; exists {
		old.Unsubscribe()
	}
	chat.subs[ctxID] = sub
	chat.mu.Unlock()

	return nil
}

// Unsubscribe removes a context's subscription
func (chat *NATSChatroom) Unsubscribe(ctxID string) {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	if sub, exists := chat.subs[ctxID]; exists {
		sub.Unsubscribe()
		delete(chat.subs, ctxID)
	}
}

// Publish sends a message to a room
func (chat *NATSChatroom) Publish(room string, msg ChatMessage) error {
	subject := "chat.room." + room
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return chat.nc.Publish(subject, data)
}

// GetHistory retrieves recent messages from JetStream
func (chat *NATSChatroom) GetHistory(room string, limit int) ([]ChatMessage, error) {
	subject := "chat.room." + room

	// Create an ephemeral consumer to replay messages
	sub, err := chat.js.SubscribeSync(subject, nats.DeliverLast())
	if err != nil {
		// No messages yet
		return nil, nil
	}
	defer sub.Unsubscribe()

	var messages []ChatMessage
	for i := 0; i < limit; i++ {
		msg, err := sub.NextMsg(100 * time.Millisecond)
		if err != nil {
			break
		}
		var chatMsg ChatMessage
		if err := json.Unmarshal(msg.Data, &chatMsg); err == nil {
			messages = append(messages, chatMsg)
		}
	}
	return messages, nil
}

func (chat *NATSChatroom) Close() {
	chat.mu.Lock()
	for _, sub := range chat.subs {
		sub.Unsubscribe()
	}
	chat.mu.Unlock()
	chat.nc.Close()
}

var roomNames = []string{"Go", "Rust", "Python", "JavaScript", "Clojure"}

func main() {
	ctx := context.Background()

	// Start embedded NATS server (JetStream enabled by default)
	ns, err := embeddednats.New(ctx,
		embeddednats.WithDirectory("./data/nats"),
	)
	if err != nil {
		log.Fatalf("Failed to start embedded NATS: %v", err)
	}
	ns.WaitForServer()

	// Get client connection to embedded server
	nc, err := ns.Client()
	if err != nil {
		log.Fatalf("Failed to connect to embedded NATS: %v", err)
	}

	chat, err := NewNATSChatroom(nc)
	if err != nil {
		log.Fatalf("Failed to initialize chatroom: %v", err)
	}
	defer chat.Close()

	v := via.New()
	v.Config(via.Options{
		DevMode:       true,
		DocumentTitle: "NATS Chat",
		LogLvl:        via.LogLevelInfo,
		ServerAddress: ":7331",
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

		// Local message cache for this context
		var messages []ChatMessage
		var messagesMu sync.Mutex
		currentRoom := "Go"

		// Context ID for subscription management
		ctxID := randID()

		// Subscribe to current room
		subscribeToRoom := func(room string) {
			chat.Subscribe(ctxID, room, func(msg *ChatMessage) {
				messagesMu.Lock()
				messages = append(messages, *msg)
				// Keep only last 50 messages
				if len(messages) > 50 {
					messages = messages[len(messages)-50:]
				}
				messagesMu.Unlock()
				c.Sync()
			})
			currentRoom = room
		}

		subscribeToRoom("Go")

		switchRoom := c.Action(func() {
			newRoom := roomSignal.String()
			if newRoom != currentRoom {
				messagesMu.Lock()
				messages = nil // Clear messages for new room
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

			chat.Publish(currentRoom, ChatMessage{
				User:    currentUser,
				Message: msg,
				Time:    time.Now().UnixMilli(),
			})
		})

		c.View(func() h.H {
			// Build room tabs
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

			// Build message list
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

func randID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
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
