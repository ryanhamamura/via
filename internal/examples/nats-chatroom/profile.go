package main

import (
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

func ProfilePage(c *via.Context) {
	existingName := c.Session().GetString(SessionKeyUsername)
	existingEmoji := c.Session().GetString(SessionKeyEmoji)
	if existingEmoji == "" {
		existingEmoji = emojiChoices[0]
	}

	nameField := c.Field(existingName,
		via.Required("Display name is required"),
		via.MinLen(2, "Must be at least 2 characters"),
		via.MaxLen(20, "Must be at most 20 characters"),
	)
	selectedEmoji := c.Signal(existingEmoji)
	previewName := c.Computed(func() string {
		if name := nameField.String(); name != "" {
			return name
		}
		return "Your Name"
	})

	saveToSession := func() bool {
		if !c.ValidateAll() {
			c.Sync()
			return false
		}
		c.Session().Set(SessionKeyUsername, nameField.String())
		c.Session().Set(SessionKeyEmoji, selectedEmoji.String())
		return true
	}

	save := c.Action(func() {
		saveToSession()
	})

	saveAndChat := c.Action(func() {
		if saveToSession() {
			c.Navigate("/", false)
		}
	})

	c.View(func() h.H {
		// Emoji grid
		emojiGrid := []h.H{h.Class("emoji-grid")}
		for _, emoji := range emojiChoices {
			cls := "emoji-option"
			if emoji == selectedEmoji.String() {
				cls += " emoji-selected"
			}
			emojiGrid = append(emojiGrid,
				h.Button(
					h.Class(cls),
					h.Type("button"),
					h.Text(emoji),
					save.OnClick(WithSignal(selectedEmoji, emoji)),
				),
			)
		}

		// Action buttons — "Start Chatting" only if editing is meaningful
		actionButtons := []h.H{h.Class("profile-actions")}
		if existingName != "" {
			actionButtons = append(actionButtons,
				h.Button(h.Text("Save"), save.OnClick(), h.Class("secondary")),
			)
		}
		actionButtons = append(actionButtons,
			h.Button(h.Text("Start Chatting"), saveAndChat.OnClick()),
		)

		return h.Div(h.Class("profile-page"),
			h.H2(h.Text("Your Profile"), h.DataViewTransition("page-title")),

			// Live preview
			h.Div(h.Class("profile-preview"),
				h.Div(h.Class("avatar avatar-lg"), h.Text(selectedEmoji.String())),
				h.Span(h.Class("preview-name"), previewName.Text()),
			),

			h.Div(h.Class("profile-form"),
				// Name field
				h.Label(h.Text("Display Name"),
					h.Input(
						h.Type("text"),
						h.Placeholder("Enter a display name"),
						nameField.Bind(),
						h.Attr("autofocus"),
						saveAndChat.OnKeyDown("Enter"),
						h.If(nameField.HasError(), h.Attr("aria-invalid", "true")),
					),
					h.If(nameField.HasError(),
						h.Small(h.Class("field-error"), h.Text(nameField.FirstError())),
					),
				),

				// Emoji picker
				h.Label(h.Text("Choose an Avatar")),
				h.Div(emojiGrid...),

				// Actions
				h.Div(actionButtons...),
			),
		)
	})
}
