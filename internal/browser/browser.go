package browser

import (
	"context"

	"github.com/adityalohuni/mcp-server/internal/page"
)

type ClickResult struct {
	Status string `json:"status"`
	Selector string `json:"selector,omitempty"`
}

type SnapshotOptions struct {
	IncludeHidden bool
	MaxElements   int
	MaxText       int
	IncludeHTML   bool
	MaxHTML       int
	MaxHTMLTokens int
}

type Browser interface {
	Click(ctx context.Context, selector string) (ClickResult, error)
	Snapshot(ctx context.Context, opts SnapshotOptions) (page.Snapshot, error)
	Scroll(ctx context.Context, opts ScrollOptions) (ScrollResult, error)
	Hover(ctx context.Context, selector string) (HoverResult, error)
	Type(ctx context.Context, selector string, text string, pressEnter bool) (TypeResult, error)
	Enter(ctx context.Context, selector string, key string) (EnterResult, error)
	Back(ctx context.Context) (HistoryResult, error)
	Forward(ctx context.Context) (HistoryResult, error)
	WaitForSelector(ctx context.Context, selector string, timeoutMs int) (WaitForSelectorResult, error)
	Find(ctx context.Context, text string, limit int, radius int, caseSensitive bool) (FindResult, error)
	Navigate(ctx context.Context, url string) (NavigateResult, error)
	Select(ctx context.Context, opts SelectOptions) (SelectResult, error)
	Screenshot(ctx context.Context, opts ScreenshotOptions) (ScreenshotResult, error)
	StartRecording(ctx context.Context) (RecordingStateResult, error)
	StopRecording(ctx context.Context) (RecordingStateResult, error)
	GetRecording(ctx context.Context) ([]RecordedAction, error)
}

type ScrollOptions struct {
	DeltaX   int
	DeltaY   int
	Selector string
	Behavior string
	Block    string
}

type ScrollResult struct {
	DeltaX   int    `json:"deltaX"`
	DeltaY   int    `json:"deltaY"`
	Selector string `json:"selector,omitempty"`
	Behavior string `json:"behavior"`
	Block    string `json:"block"`
}

type HoverResult struct {
	Selector string `json:"selector"`
}

type TypeResult struct {
	Selector   string `json:"selector"`
	TextLength int    `json:"textLength"`
	PressEnter bool   `json:"pressEnter"`
}

type EnterResult struct {
	Selector         string `json:"selector,omitempty"`
	Key              string `json:"key"`
	UsedActiveElement bool  `json:"usedActiveElement"`
}

type HistoryResult struct {
	Direction string `json:"direction"`
}

type WaitForSelectorResult struct {
	Selector  string `json:"selector"`
	TimeoutMs int    `json:"timeoutMs"`
	Found     bool   `json:"found"`
}

type FindResultItem struct {
	Index   int    `json:"index"`
	Snippet string `json:"snippet"`
}

type FindResult struct {
	Query         string           `json:"query"`
	Limit         int              `json:"limit"`
	Radius        int              `json:"radius"`
	CaseSensitive bool             `json:"caseSensitive"`
	Total         int              `json:"total"`
	Returned      int              `json:"returned"`
	Results       []FindResultItem `json:"results"`
}

type NavigateResult struct {
	URL string `json:"url"`
}

type SelectOptions struct {
	Selector  string
	Value     string
	Label     string
	Index     int
	Values    []string
	Labels    []string
	Indices   []int
	MatchMode string
	Toggle    bool
}

type SelectResult struct {
	Selector      string   `json:"selector"`
	Value         string   `json:"value"`
	Label         string   `json:"label,omitempty"`
	Index         int      `json:"index,omitempty"`
	Values        []string `json:"values,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	Indices       []int    `json:"indices,omitempty"`
	MatchMode     string   `json:"matchMode,omitempty"`
	Toggle        bool     `json:"toggle,omitempty"`
	Multiple      bool     `json:"multiple,omitempty"`
	SelectedCount int      `json:"selectedCount,omitempty"`
}

type ScreenshotOptions struct {
	Selector  string
	Padding   int
	Format    string
	Quality   float64
	MaxWidth  int
	MaxHeight int
}

type ScreenshotResult struct {
	Selector string `json:"selector"`
	DataURL  string `json:"dataUrl"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
}

type RecordingStateResult struct {
	Recording bool `json:"recording"`
	Count     int  `json:"count"`
}

type RecordedAction struct {
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	Timestamp int64          `json:"timestamp"`
	URL       string         `json:"url"`
	Title     string         `json:"title"`
}
