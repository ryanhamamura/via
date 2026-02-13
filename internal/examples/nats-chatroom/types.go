package main

import "github.com/ryanhamamura/via/h"

const (
	SessionKeyUsername = "username"
	SessionKeyEmoji    = "emoji"
)

type ChatMessage struct {
	User    UserInfo `json:"user"`
	Message string   `json:"message"`
	Time    int64    `json:"time"`
}

type UserInfo struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

func (u *UserInfo) Avatar() h.H {
	return h.Div(h.Class("avatar"), h.Attr("title", u.Name), h.Text(u.Emoji))
}

var roomNames = []string{"Go", "Rust", "Python", "JavaScript", "Clojure"}

var emojiChoices = []string{
	"🐼", "🐯", "🦅", "🐬", "🦊", "🐺", "🐻", "🦦", "🦁", "🐸",
	"🦄", "🐙", "🦀", "🐝", "🦋", "🐢", "🦉", "🐳", "🦈", "🐧",
}
