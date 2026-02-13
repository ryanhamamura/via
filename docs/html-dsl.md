# HTML DSL

Reference for the `h` package — Via's HTML builder.

## Overview

The `h` package wraps [gomponents](https://github.com/maragudk/gomponents) with a single interface:

```go
type H interface {
    Render(w io.Writer) error
}
```

Every element, attribute, and text node implements `H`. Build HTML by nesting function calls:

```go
import "github.com/ryanhamamura/via/h"

h.Div(h.Class("card"),
    h.H2(h.Text("Title")),
    h.P(h.Textf("Count: %d", count)),
    h.Button(h.Text("Click"), action.OnClick()),
)
```

For cleaner templates, use a dot import:

```go
import . "github.com/ryanhamamura/via/h"

Div(Class("card"),
    H2(Text("Title")),
    P(Textf("Count: %d", count)),
    Button(Text("Click"), action.OnClick()),
)
```

## Text Nodes

| Function | Description |
|----------|-------------|
| `Text(s)` | Escaped text node |
| `Textf(fmt, args...)` | Escaped text with `fmt.Sprintf` |
| `Raw(s)` | Unescaped raw HTML — use for trusted content like SVG |
| `Rawf(fmt, args...)` | Unescaped raw HTML with `fmt.Sprintf` |

## Elements

Every element function takes `...H` children (elements, attributes, and text nodes mixed together) except `Style(v string)` and `Title(v string)` which take a single string.

### Document structure

`HTML`, `Head`, `Body`, `Main`, `Header`, `Footer`, `Section`, `Article`, `Aside`, `Nav`, `Div`, `Span`

### Headings

`H1`, `H2`, `H3`, `H4`, `H5`, `H6`

### Text

`P`, `A`, `Strong`, `Em`, `B`, `I`, `U`, `S`, `Small`, `Mark`, `Del`, `Ins`, `Sub`, `Sup`, `Abbr`, `Cite`, `Code`, `Pre`, `Samp`, `Kbd`, `Var`, `Q`, `BlockQuote`, `Dfn`, `Wbr`, `Br`, `Hr`

### Forms

`Form`, `Input`, `Textarea`, `Select`, `Option`, `OptGroup`, `Button`, `Label`, `FieldSet`, `Legend`, `DataList`, `Meter`, `Progress`

### Tables

`Table`, `THead`, `TBody`, `TFoot`, `Tr`, `Th`, `Td`, `Caption`, `Col`, `ColGroup`

### Lists

`Ul`, `Ol`, `Li`, `Dl`, `Dt`, `Dd`

### Media

`Img`, `Audio`, `Video`, `Source`, `Picture`, `Canvas`, `IFrame`, `Embed`, `Object`

### Other

`Details`, `Summary`, `Dialog`, `Template`, `NoScript`, `Figure`, `FigCaption`, `Address`, `Time`, `Base`, `Link`, `Meta`, `Script`, `Area`

### Special signatures

| Function | Signature | Notes |
|----------|-----------|-------|
| `Style(v)` | `func Style(v string) H` | Inline `style` attribute, not a container element |
| `StyleEl(children...)` | `func StyleEl(children ...H) H` | The `<style>` element as a container |
| `Title(v)` | `func Title(v string) H` | Sets `<title>` text |

## Attributes

### Generic

```go
Attr("name", "value")     // name="value"
Attr("disabled")           // boolean attribute (no value)
```

`Attr` with no value produces a boolean attribute. With one value, it produces a name-value pair. More than one value panics.

### Named helpers

| Function | HTML output |
|----------|-------------|
| `ID(v)` | `id="v"` |
| `Class(v)` | `class="v"` |
| `Href(v)` | `href="v"` |
| `Src(v)` | `src="v"` |
| `Type(v)` | `type="v"` |
| `Value(v)` | `value="v"` |
| `Placeholder(v)` | `placeholder="v"` |
| `Rel(v)` | `rel="v"` |
| `Role(v)` | `role="v"` |
| `Data(name, v)` | `data-name="v"` (auto-prefixes `data-`) |

## Conditional Rendering

```go
h.If(showError, h.P(h.Class("error"), h.Text("Something went wrong")))
```

Returns the node when `true`, `nil` (renders nothing) when `false`.

## Datastar Helpers

These produce attributes used by Datastar for client-side reactivity.

| Function | Output | Description |
|----------|--------|-------------|
| `DataInit(expr)` | `data-init="expr"` | Initialize client-side state |
| `DataEffect(expr)` | `data-effect="expr"` | Reactive side effect expression |
| `DataIgnoreMorph()` | `data-ignore-morph` | Skip this element during DOM morph. See [SPA Navigation](routing-and-navigation.md#dataignoremorph) |
| `DataViewTransition(name)` | `style="view-transition-name: name"` | Animate element across SPA navigations. See [View Transitions](routing-and-navigation.md#view-transitions) |

> `DataViewTransition` sets the entire `style` attribute. If you also need other inline styles, include `view-transition-name` directly in a `Style()` call.

## Utilities

### HTML5

Full HTML5 document template:

```go
h.HTML5(h.HTML5Props{
    Title:       "My Page",
    Description: "Page description",
    Language:    "en",
    Head:        []h.H{h.Link(h.Rel("stylesheet"), h.Href("/style.css"))},
    Body:        []h.H{h.Div(h.Text("Hello"))},
})
```

Via uses this internally to render the initial page document. You typically don't need it directly.

### JoinAttrs

Joins attribute values from child nodes by spaces:

```go
h.JoinAttrs("class", h.Class("card"), h.Class("active"))
// → class="card active"
```
