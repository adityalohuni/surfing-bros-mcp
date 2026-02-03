package protocol

import "encoding/json"

type CommandType string

const (
	CommandClick          CommandType = "click"
	CommandSnapshot       CommandType = "snapshot"
	CommandScroll         CommandType = "scroll"
	CommandHover          CommandType = "hover"
	CommandTypeText       CommandType = "type"
	CommandEnter          CommandType = "enter"
	CommandBack           CommandType = "back"
	CommandForward        CommandType = "forward"
	CommandWaitFor        CommandType = "waitForSelector"
	CommandFind           CommandType = "find"
	CommandNavigate       CommandType = "navigate"
	CommandSelect         CommandType = "select"
	CommandScreenshot     CommandType = "screenshot"
	CommandStartRecording CommandType = "start_recording"
	CommandStopRecording  CommandType = "stop_recording"
	CommandGetRecording   CommandType = "get_recording"
	CommandListTabs       CommandType = "list_tabs"
	CommandOpenTab        CommandType = "open_tab"
	CommandCloseTab       CommandType = "close_tab"
	CommandClaimTab       CommandType = "claim_tab"
	CommandReleaseTab     CommandType = "release_tab"
	CommandSetTabSharing  CommandType = "set_tab_sharing"
)

type Command struct {
	ID        string          `json:"id"`
	Type      CommandType     `json:"type"`
	SessionID string          `json:"sessionId,omitempty"`
	TabID     int             `json:"tabId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type Response struct {
	ID        string          `json:"id"`
	OK        bool            `json:"ok"`
	Error     string          `json:"error,omitempty"`
	ErrorCode string          `json:"errorCode,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type ClickPayload struct {
	Selector string `json:"selector"`
}

type SnapshotPayload struct {
	IncludeHidden bool `json:"includeHidden,omitempty"`
	MaxElements   int  `json:"maxElements,omitempty"`
	MaxText       int  `json:"maxText,omitempty"`
	IncludeHTML   bool `json:"includeHTML,omitempty"`
	MaxHTML       int  `json:"maxHTML,omitempty"`
	MaxHTMLTokens int  `json:"maxHTMLTokens,omitempty"`
}

type ScrollPayload struct {
	DeltaX   int    `json:"deltaX,omitempty"`
	DeltaY   int    `json:"deltaY,omitempty"`
	Selector string `json:"selector,omitempty"`
	Behavior string `json:"behavior,omitempty"`
	Block    string `json:"block,omitempty"`
}

type HoverPayload struct {
	Selector string `json:"selector"`
}

type TypePayload struct {
	Selector   string `json:"selector"`
	Text       string `json:"text"`
	PressEnter bool   `json:"pressEnter,omitempty"`
}

type EnterPayload struct {
	Selector string `json:"selector,omitempty"`
	Key      string `json:"key,omitempty"`
}

type NavigatePayload struct {
	URL string `json:"url"`
}

type FindPayload struct {
	Text          string `json:"text"`
	Limit         int    `json:"limit,omitempty"`
	Radius        int    `json:"radius,omitempty"`
	CaseSensitive bool   `json:"caseSensitive,omitempty"`
}

type WaitForSelectorPayload struct {
	Selector  string `json:"selector"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type SelectPayload struct {
	Selector  string   `json:"selector"`
	Value     string   `json:"value,omitempty"`
	Label     string   `json:"label,omitempty"`
	Index     int      `json:"index,omitempty"`
	Values    []string `json:"values,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Indices   []int    `json:"indices,omitempty"`
	MatchMode string   `json:"matchMode,omitempty"`
	Toggle    bool     `json:"toggle,omitempty"`
}

type ScreenshotPayload struct {
	Selector  string  `json:"selector,omitempty"`
	Padding   int     `json:"padding,omitempty"`
	Format    string  `json:"format,omitempty"`
	Quality   float64 `json:"quality,omitempty"`
	MaxWidth  int     `json:"maxWidth,omitempty"`
	MaxHeight int     `json:"maxHeight,omitempty"`
}

type OpenTabPayload struct {
	URL    string `json:"url,omitempty"`
	Active bool   `json:"active,omitempty"`
	Pinned bool   `json:"pinned,omitempty"`
}

type CloseTabPayload struct {
	TabID int `json:"tabId"`
}

type ClaimTabPayload struct {
	TabID         int    `json:"tabId"`
	Mode          string `json:"mode,omitempty"`
	RequireActive bool   `json:"requireActive,omitempty"`
}

type ReleaseTabPayload struct {
	TabID int `json:"tabId"`
}

type SetTabSharingPayload struct {
	TabID       int  `json:"tabId"`
	AllowShared bool `json:"allowShared"`
}

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

type SnapshotData struct {
	URL      string    `json:"url"`
	Title    string    `json:"title,omitempty"`
	Text     string    `json:"text,omitempty"`
	HTML     string    `json:"html,omitempty"`
	Elements []Element `json:"elements,omitempty"`
}
