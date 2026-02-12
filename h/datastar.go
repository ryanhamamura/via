package h

func DataInit(expression string) H {
	return Data("init", expression)
}

func DataEffect(expression string) H {
	return Data("effect", expression)
}

func DataIgnoreMorph() H {
	return Attr("data-ignore-morph")
}

// DataViewTransition sets the view-transition-name CSS property on an element
// via an inline style. Elements with matching names animate between pages
// during SPA navigation.
func DataViewTransition(name string) H {
	return Attr("style", "view-transition-name: "+name)
}
