package page

import (
	"strings"

	"golang.org/x/net/html"
)

const (
	defaultMaxText     = 4000
	defaultMaxElements = 80
)

type ReduceOptions struct {
	MaxText     int
	MaxElements int
}

type Reducer struct {
	maxText     int
	maxElements int
}

func NewReducer(opts ReduceOptions) *Reducer {
	maxText := opts.MaxText
	if maxText <= 0 {
		maxText = defaultMaxText
	}
	maxElements := opts.MaxElements
	if maxElements <= 0 {
		maxElements = defaultMaxElements
	}
	return &Reducer{maxText: maxText, maxElements: maxElements}
}

func (r *Reducer) Reduce(raw RawPage) Snapshot {
	text := strings.TrimSpace(raw.Text)
	var elements []Element
	if raw.HTML != "" {
		parsedText, parsedElements := parseHTML(raw.HTML, r.maxElements)
		if text == "" {
			text = parsedText
		}
		if len(raw.Elements) == 0 {
			elements = parsedElements
		}
	}
	if len(elements) == 0 {
		elements = raw.Elements
	}

	text = compactWhitespace(text)
	if len(text) > r.maxText {
		text = text[:r.maxText]
	}
	if len(elements) > r.maxElements {
		elements = elements[:r.maxElements]
	}

	actions := buildActions(elements)

	return Snapshot{
		URL:      raw.URL,
		Title:    raw.Title,
		Text:     text,
		Elements: elements,
		Actions:  actions,
	}
}

func parseHTML(htmlText string, maxElements int) (string, []Element) {
	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return stripHTML(htmlText), nil
	}
	var elements []Element
	var b strings.Builder
	var walk func(n *html.Node, path []string)
	walk = func(n *html.Node, path []string) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			path = append(path, tag)
			if isActionable(tag, n) {
				el := elementFromNode(tag, n, path)
				if el.Text != "" || el.ARIALabel != "" || el.Name != "" || el.ID != "" {
					elements = append(elements, el)
				}
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if maxElements > 0 && len(elements) >= maxElements {
				break
			}
			walk(c, path)
		}
	}
	walk(doc, nil)

	return b.String(), elements
}

func isActionable(tag string, n *html.Node) bool {
	switch tag {
	case "a", "button", "input", "select", "textarea":
		return true
	case "div", "span":
		return attr(n, "role") == "button" || attr(n, "role") == "link"
	default:
		return false
	}
}

func elementFromNode(tag string, n *html.Node, path []string) Element {
	el := Element{
		Tag:         tag,
		Text:        nodeText(n),
		Selector:    selectorFromNode(tag, n, path),
		Href:        attr(n, "href"),
		InputType:   attr(n, "type"),
		Name:        attr(n, "name"),
		ID:          attr(n, "id"),
		ARIALabel:   attr(n, "aria-label"),
		Title:       attr(n, "title"),
		Alt:         attr(n, "alt"),
		Value:       attr(n, "value"),
		Placeholder: attr(n, "placeholder"),
		Context:     contextText(n, 80),
	}
	return el
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteByte(' ')
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func selectorFromNode(tag string, n *html.Node, path []string) string {
	if id := attr(n, "id"); id != "" {
		return "#" + id
	}
	if v := attr(n, "data-testid"); v != "" {
		return tag + "[data-testid=\"" + v + "\"]"
	}
	if v := firstDataAttr(n, []string{"data-test", "data-qa", "data-automation", "data-cy", "data-automation-id"}); v.key != "" {
		return tag + "[" + v.key + "=\"" + v.val + "\"]"
	}
	if v := attr(n, "name"); v != "" {
		return tag + "[name=\"" + v + "\"]"
	}
	if v := attr(n, "aria-label"); v != "" {
		return tag + "[aria-label=\"" + v + "\"]"
	}
	class := attr(n, "class")
	if class != "" {
		parts := strings.Fields(class)
		if len(parts) > 0 {
			return tag + "." + parts[0]
		}
	}
	index := nthChildIndex(n)
	if index > 0 {
		return tag + ":nth-child(" + itoa(index) + ")"
	}
	if len(path) > 0 {
		return strings.Join(path, " > ")
	}
	return tag
}

func nthChildIndex(n *html.Node) int {
	if n == nil || n.Parent == nil {
		return 0
	}
	idx := 0
	for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		idx++
		if c == n {
			return idx
		}
	}
	return 0
}

type dataAttr struct {
	key string
	val string
}

func firstDataAttr(n *html.Node, keys []string) dataAttr {
	for _, key := range keys {
		if v := attr(n, key); v != "" {
			return dataAttr{key: key, val: v}
		}
	}
	return dataAttr{}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func compactWhitespace(input string) string {
	parts := strings.Fields(input)
	return strings.Join(parts, " ")
}

func stripHTML(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	inTag := false
	for i := 0; i < len(input); i++ {
		s := input[i]
		switch s {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteByte(s)
			}
		}
	}
	return b.String()
}

func buildActions(elements []Element) []Action {
	if len(elements) == 0 {
		return nil
	}
	actions := make([]Action, 0, len(elements))
	for _, el := range elements {
		verb := actionVerb(el)
		if verb == "" || el.Selector == "" {
			continue
		}
		label := actionLabel(el)
		actions = append(actions, Action{
			Verb:     verb,
			Selector: el.Selector,
			Label:    label,
			Hint:     actionHint(el),
		})
	}
	return actions
}

func actionLabel(el Element) string {
	for _, v := range []string{strings.TrimSpace(el.Text), el.ARIALabel, el.Title, el.Alt, el.Value, el.Placeholder, el.Name, el.Context} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func actionVerb(el Element) string {
	switch el.Tag {
	case "a":
		return "open"
	case "button":
		return "click"
	case "input":
		switch strings.ToLower(el.InputType) {
		case "submit", "button":
			return "click"
		case "checkbox":
			return "toggle"
		case "radio":
			return "select"
		default:
			return "type"
		}
	case "select":
		return "select"
	case "textarea":
		return "type"
	case "div", "span":
		return "click"
	default:
		return ""
	}
}

func actionHint(el Element) string {
	if el.Href != "" {
		return "href=" + el.Href
	}
	if el.InputType != "" {
		return "type=" + el.InputType
	}
	if el.Name != "" {
		return "name=" + el.Name
	}
	return ""
}

func contextText(n *html.Node, limit int) string {
	if n == nil || n.Parent == nil {
		return ""
	}
	var b strings.Builder
	for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
		if c == n {
			continue
		}
		if c.Type == html.TextNode {
			text := strings.TrimSpace(c.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteByte(' ')
			}
		}
	}
	ctx := strings.TrimSpace(b.String())
	if limit > 0 && len(ctx) > limit {
		ctx = ctx[:limit]
	}
	return ctx
}
