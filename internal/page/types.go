package page

type Element struct {
	Tag         string `json:"tag,omitempty"`
	Text        string `json:"text,omitempty"`
	Selector    string `json:"selector,omitempty"`
	Href        string `json:"href,omitempty"`
	InputType   string `json:"inputType,omitempty"`
	Name        string `json:"name,omitempty"`
	ID          string `json:"id,omitempty"`
	ARIALabel   string `json:"ariaLabel,omitempty"`
	Title       string `json:"title,omitempty"`
	Alt         string `json:"alt,omitempty"`
	Value       string `json:"value,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Context     string `json:"context,omitempty"`
}

type Snapshot struct {
	ID       string    `json:"id"`
	URL      string    `json:"url"`
	Title    string    `json:"title,omitempty"`
	Text     string    `json:"text,omitempty"`
	Elements []Element `json:"elements,omitempty"`
	Actions  []Action  `json:"actions,omitempty"`
}

type Action struct {
	Verb     string `json:"verb"`
	Selector string `json:"selector"`
	Label    string `json:"label,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

type RawPage struct {
	URL      string
	Title    string
	Text     string
	HTML     string
	Elements []Element
}
