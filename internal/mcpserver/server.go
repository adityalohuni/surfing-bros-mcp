package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adityalohuni/mcp-server/internal/browser"
	"github.com/adityalohuni/mcp-server/internal/page"
	"github.com/adityalohuni/mcp-server/internal/workflow"
)

type Options struct {
	Implementation *mcp.Implementation
	Instructions   string
	WorkflowLimit  int
}

type Server struct {
	mcpServer *mcp.Server
	browser   browser.Browser
	store     *page.Store
	workflows *workflow.Store
	workflowLimit int
}

func New(browserClient browser.Browser, store *page.Store, opts Options) *Server {
	impl := opts.Implementation
	if impl == nil {
		impl = &mcp.Implementation{Name: "surfingbro-browser", Version: "v1.0.0"}
	}
	if store == nil {
		store = page.NewStore()
	}
	workflows := workflow.NewStore("workflows.json")
	server := mcp.NewServer(impl, &mcp.ServerOptions{Instructions: opts.Instructions})
	s := &Server{mcpServer: server, browser: browserClient, store: store, workflows: workflows, workflowLimit: opts.WorkflowLimit}
	if opts.WorkflowLimit > 0 {
		_, _ = workflows.Compact(opts.WorkflowLimit)
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.click",
		Description: "Click the first element matching a CSS selector on the active page.",
	}, s.click)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.snapshot",
		Description: "Return a reduced, LLM-friendly snapshot of the current page.",
	}, s.snapshot)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.scroll",
		Description: "Scroll the page or a specific element by pixel offsets.",
	}, s.scroll)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.hover",
		Description: "Hover over the first element matching a CSS selector.",
	}, s.hover)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.type",
		Description: "Type text into an input or textarea; optionally press Enter.",
	}, s.typeText)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.enter",
		Description: "Press a key (default Enter) on a target element or active element.",
	}, s.enter)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.back",
		Description: "Navigate backward in browser history.",
	}, s.back)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.forward",
		Description: "Navigate forward in browser history.",
	}, s.forward)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.wait_for_selector",
		Description: "Wait for a selector to appear in the DOM.",
	}, s.waitForSelector)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.find",
		Description: "Find text on the page and return short snippets.",
	}, s.find)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.navigate",
		Description: "Navigate to a URL in the active tab.",
	}, s.navigate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.select",
		Description: "Select option(s) in a <select> by value/label/index.",
	}, s.selectOption)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.screenshot",
		Description: "Capture a screenshot of an element or the viewport.",
	}, s.screenshot)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.start_recording",
		Description: "Start recording user actions in the browser.",
	}, s.startRecording)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.stop_recording",
		Description: "Stop recording user actions in the browser.",
	}, s.stopRecording)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser.get_recording",
		Description: "Get the current recorded action list.",
	}, s.getRecording)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow.save",
		Description: "Save a recorded workflow into server memory.",
	}, s.saveWorkflow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow.compact",
		Description: "Compact workflow memory to a maximum count.",
	}, s.compactWorkflows)

	server.AddResource(&mcp.Resource{
		Name:        "browser_latest",
		Description: "Read the most recent stored page snapshot.",
		URI:         "browser://page/latest",
		MIMEType:    "application/json",
	}, s.readLatest)

	server.AddResource(&mcp.Resource{
		Name:        "workflow_list",
		Description: "List saved workflows.",
		URI:         "workflow://list",
		MIMEType:    "application/json",
	}, s.readWorkflowList)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "browser_page",
		Description: "Read a stored page snapshot by ID.",
		URITemplate: "browser://page/{snapshot_id}",
		MIMEType:    "application/json",
	}, s.readSnapshot)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "workflow_item",
		Description: "Read a workflow by ID.",
		URITemplate: "workflow://{workflow_id}",
		MIMEType:    "application/json",
	}, s.readWorkflow)

	return s
}

func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.mcpServer.Run(ctx, transport)
}

type ClickInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector for the element to click"`
}

type ClickOutput struct {
	Status string `json:"status" jsonschema:"status of the click operation"`
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector that was clicked"`
}

func (s *Server) click(ctx context.Context, _ *mcp.CallToolRequest, input ClickInput) (*mcp.CallToolResult, ClickOutput, error) {
	if input.Selector == "" {
		return nil, ClickOutput{}, errors.New("selector is required")
	}
	result, err := s.browser.Click(ctx, input.Selector)
	if err != nil {
		return nil, ClickOutput{}, err
	}
	return nil, ClickOutput{Status: result.Status, Selector: result.Selector}, nil
}

type SnapshotInput struct {
	IncludeHidden bool `json:"includeHidden,omitempty" jsonschema:"include hidden elements"`
	MaxElements   int  `json:"maxElements,omitempty" jsonschema:"maximum number of elements to return"`
	MaxText       int  `json:"maxText,omitempty" jsonschema:"maximum characters of text to return"`
	IncludeHTML   bool `json:"includeHTML,omitempty" jsonschema:"include raw HTML in snapshot"`
	MaxHTML       int  `json:"maxHTML,omitempty" jsonschema:"max characters of HTML to return"`
	MaxHTMLTokens int  `json:"maxHTMLTokens,omitempty" jsonschema:"approx max HTML tokens to return"`
}

type SnapshotOutput struct {
	SnapshotID string        `json:"snapshot_id" jsonschema:"identifier for the stored snapshot"`
	URL        string        `json:"url" jsonschema:"page URL"`
	Title      string        `json:"title,omitempty" jsonschema:"page title"`
	Text       string        `json:"text" jsonschema:"reduced page text"`
	Elements   []page.Element `json:"elements,omitempty" jsonschema:"actionable elements"`
	Actions    []page.Action  `json:"actions,omitempty" jsonschema:"compact action map"`
}

func (s *Server) snapshot(ctx context.Context, _ *mcp.CallToolRequest, input SnapshotInput) (*mcp.CallToolResult, SnapshotOutput, error) {
	snap, err := s.browser.Snapshot(ctx, browser.SnapshotOptions{
		IncludeHidden: input.IncludeHidden,
		MaxElements:   input.MaxElements,
		MaxText:       input.MaxText,
		IncludeHTML:   input.IncludeHTML,
		MaxHTML:       input.MaxHTML,
		MaxHTMLTokens: input.MaxHTMLTokens,
	})
	if err != nil {
		return nil, SnapshotOutput{}, err
	}
	if snap.ID == "" {
		snap.ID = s.store.Put(snap)
	}
	return nil, SnapshotOutput{
		SnapshotID: snap.ID,
		URL:        snap.URL,
		Title:      snap.Title,
		Text:       snap.Text,
		Elements:   snap.Elements,
		Actions:    snap.Actions,
	}, nil
}

type ScrollInput struct {
	DeltaX   int    `json:"deltaX,omitempty" jsonschema:"horizontal scroll delta in pixels"`
	DeltaY   int    `json:"deltaY,omitempty" jsonschema:"vertical scroll delta in pixels"`
	Selector string `json:"selector,omitempty" jsonschema:"optional selector for a scrollable element"`
	Behavior string `json:"behavior,omitempty" jsonschema:"scroll behavior: auto or smooth"`
	Block    string `json:"block,omitempty" jsonschema:"scroll alignment: start|center|end|nearest"`
}

func (s *Server) scroll(ctx context.Context, _ *mcp.CallToolRequest, input ScrollInput) (*mcp.CallToolResult, browser.ScrollResult, error) {
	out, err := s.browser.Scroll(ctx, browser.ScrollOptions{
		DeltaX:   input.DeltaX,
		DeltaY:   input.DeltaY,
		Selector: input.Selector,
		Behavior: input.Behavior,
		Block:    input.Block,
	})
	if err != nil {
		return nil, browser.ScrollResult{}, err
	}
	return nil, out, nil
}

type HoverInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector of element to hover"`
}

func (s *Server) hover(ctx context.Context, _ *mcp.CallToolRequest, input HoverInput) (*mcp.CallToolResult, browser.HoverResult, error) {
	out, err := s.browser.Hover(ctx, input.Selector)
	if err != nil {
		return nil, browser.HoverResult{}, err
	}
	return nil, out, nil
}

type TypeInput struct {
	Selector   string `json:"selector" jsonschema:"CSS selector of input/textarea"`
	Text       string `json:"text" jsonschema:"text to enter"`
	PressEnter bool   `json:"pressEnter,omitempty" jsonschema:"press Enter after typing"`
}

func (s *Server) typeText(ctx context.Context, _ *mcp.CallToolRequest, input TypeInput) (*mcp.CallToolResult, browser.TypeResult, error) {
	out, err := s.browser.Type(ctx, input.Selector, input.Text, input.PressEnter)
	if err != nil {
		return nil, browser.TypeResult{}, err
	}
	return nil, out, nil
}

type EnterInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"optional selector to send key to"`
	Key      string `json:"key,omitempty" jsonschema:"key to send (default Enter)"`
}

func (s *Server) enter(ctx context.Context, _ *mcp.CallToolRequest, input EnterInput) (*mcp.CallToolResult, browser.EnterResult, error) {
	out, err := s.browser.Enter(ctx, input.Selector, input.Key)
	if err != nil {
		return nil, browser.EnterResult{}, err
	}
	return nil, out, nil
}

type EmptyInput struct{}

func (s *Server) back(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, browser.HistoryResult, error) {
	out, err := s.browser.Back(ctx)
	if err != nil {
		return nil, browser.HistoryResult{}, err
	}
	return nil, out, nil
}

func (s *Server) forward(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, browser.HistoryResult, error) {
	out, err := s.browser.Forward(ctx)
	if err != nil {
		return nil, browser.HistoryResult{}, err
	}
	return nil, out, nil
}

type WaitForSelectorInput struct {
	Selector  string `json:"selector" jsonschema:"CSS selector to wait for"`
	TimeoutMs int    `json:"timeoutMs,omitempty" jsonschema:"timeout in milliseconds"`
}

func (s *Server) waitForSelector(ctx context.Context, _ *mcp.CallToolRequest, input WaitForSelectorInput) (*mcp.CallToolResult, browser.WaitForSelectorResult, error) {
	out, err := s.browser.WaitForSelector(ctx, input.Selector, input.TimeoutMs)
	if err != nil {
		return nil, browser.WaitForSelectorResult{}, err
	}
	return nil, out, nil
}

type FindInput struct {
	Text          string `json:"text" jsonschema:"text to search for"`
	Limit         int    `json:"limit,omitempty" jsonschema:"max results returned"`
	Radius        int    `json:"radius,omitempty" jsonschema:"context radius for snippets"`
	CaseSensitive bool   `json:"caseSensitive,omitempty" jsonschema:"case sensitive search"`
}

func (s *Server) find(ctx context.Context, _ *mcp.CallToolRequest, input FindInput) (*mcp.CallToolResult, browser.FindResult, error) {
	out, err := s.browser.Find(ctx, input.Text, input.Limit, input.Radius, input.CaseSensitive)
	if err != nil {
		return nil, browser.FindResult{}, err
	}
	return nil, out, nil
}

type NavigateInput struct {
	URL string `json:"url" jsonschema:"URL to navigate to"`
}

func (s *Server) navigate(ctx context.Context, _ *mcp.CallToolRequest, input NavigateInput) (*mcp.CallToolResult, browser.NavigateResult, error) {
	out, err := s.browser.Navigate(ctx, input.URL)
	if err != nil {
		return nil, browser.NavigateResult{}, err
	}
	return nil, out, nil
}

type SelectInput struct {
	Selector  string   `json:"selector" jsonschema:"CSS selector for select element"`
	Value     string   `json:"value,omitempty" jsonschema:"option value to select"`
	Label     string   `json:"label,omitempty" jsonschema:"option label to select"`
	Index     int      `json:"index,omitempty" jsonschema:"option index to select"`
	Values    []string `json:"values,omitempty" jsonschema:"values to select (multi-select)"`
	Labels    []string `json:"labels,omitempty" jsonschema:"labels to select (multi-select)"`
	Indices   []int    `json:"indices,omitempty" jsonschema:"indices to select (multi-select)"`
	MatchMode string   `json:"matchMode,omitempty" jsonschema:"label match mode: exact or partial"`
	Toggle    bool     `json:"toggle,omitempty" jsonschema:"toggle selection (multi-select)"`
}

func (s *Server) selectOption(ctx context.Context, _ *mcp.CallToolRequest, input SelectInput) (*mcp.CallToolResult, browser.SelectResult, error) {
	out, err := s.browser.Select(ctx, browser.SelectOptions{
		Selector:  input.Selector,
		Value:     input.Value,
		Label:     input.Label,
		Index:     input.Index,
		Values:    input.Values,
		Labels:    input.Labels,
		Indices:   input.Indices,
		MatchMode: input.MatchMode,
		Toggle:    input.Toggle,
	})
	if err != nil {
		return nil, browser.SelectResult{}, err
	}
	return nil, out, nil
}

type ScreenshotInput struct {
	Selector  string  `json:"selector,omitempty" jsonschema:"element selector (omit for viewport)"`
	Padding   int     `json:"padding,omitempty" jsonschema:"padding around element in pixels"`
	Format    string  `json:"format,omitempty" jsonschema:"png or jpeg"`
	Quality   float64 `json:"quality,omitempty" jsonschema:"jpeg quality 0-1"`
	MaxWidth  int     `json:"maxWidth,omitempty" jsonschema:"max output width"`
	MaxHeight int     `json:"maxHeight,omitempty" jsonschema:"max output height"`
}

func (s *Server) screenshot(ctx context.Context, _ *mcp.CallToolRequest, input ScreenshotInput) (*mcp.CallToolResult, browser.ScreenshotResult, error) {
	out, err := s.browser.Screenshot(ctx, browser.ScreenshotOptions{
		Selector:  input.Selector,
		Padding:   input.Padding,
		Format:    input.Format,
		Quality:   input.Quality,
		MaxWidth:  input.MaxWidth,
		MaxHeight: input.MaxHeight,
	})
	if err != nil {
		return nil, browser.ScreenshotResult{}, err
	}
	return nil, out, nil
}

func (s *Server) readSnapshot(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if req == nil || req.Params == nil {
		return nil, errors.New("missing resource params")
	}
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid resource URI: %w", err)
	}
	if u.Scheme != "browser" || u.Host != "page" {
		return nil, fmt.Errorf("unsupported resource URI: %s", req.Params.URI)
	}
	id := strings.TrimPrefix(u.Path, "/")
	if id == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	snap, ok := s.store.Get(id)
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func (s *Server) readLatest(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if req == nil || req.Params == nil {
		return nil, errors.New("missing resource params")
	}
	snap, ok := s.store.Latest()
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

type RecordingStateOutput struct {
	Recording bool `json:"recording"`
	Count     int  `json:"count"`
}

func (s *Server) startRecording(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, RecordingStateOutput, error) {
	out, err := s.browser.StartRecording(ctx)
	if err != nil {
		return nil, RecordingStateOutput{}, err
	}
	return nil, RecordingStateOutput{Recording: out.Recording, Count: out.Count}, nil
}

func (s *Server) stopRecording(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, RecordingStateOutput, error) {
	out, err := s.browser.StopRecording(ctx)
	if err != nil {
		return nil, RecordingStateOutput{}, err
	}
	return nil, RecordingStateOutput{Recording: out.Recording, Count: out.Count}, nil
}

type RecordingOutput struct {
	Actions []browser.RecordedAction `json:"actions"`
}

func (s *Server) getRecording(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, RecordingOutput, error) {
	out, err := s.browser.GetRecording(ctx)
	if err != nil {
		return nil, RecordingOutput{}, err
	}
	return nil, RecordingOutput{Actions: out}, nil
}

type WorkflowSaveInput struct {
	Name        string                  `json:"name" jsonschema:"workflow name"`
	Description string                  `json:"description,omitempty" jsonschema:"workflow description"`
	Steps       []browser.RecordedAction `json:"steps,omitempty" jsonschema:"recorded steps"`
}

type WorkflowSaveOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	StepCount   int    `json:"stepCount"`
}

func (s *Server) saveWorkflow(ctx context.Context, _ *mcp.CallToolRequest, input WorkflowSaveInput) (*mcp.CallToolResult, WorkflowSaveOutput, error) {
	steps := input.Steps
	if len(steps) == 0 {
		recording, err := s.browser.GetRecording(ctx)
		if err != nil {
			return nil, WorkflowSaveOutput{}, err
		}
		steps = recording
	}
	if len(steps) == 0 {
		return nil, WorkflowSaveOutput{}, errors.New("no steps to save")
	}
	if input.Name == "" {
		input.Name = "workflow"
	}
	w := s.workflows.Add(workflow.Workflow{
		Name:        input.Name,
		Description: input.Description,
		Steps:       steps,
	})
	if s.workflowLimit > 0 {
		_, _ = s.workflows.Compact(s.workflowLimit)
	}
	return nil, WorkflowSaveOutput{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		StepCount:   len(w.Steps),
	}, nil
}

type WorkflowCompactInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"max workflows to keep"`
}

type WorkflowCompactOutput struct {
	Removed int `json:"removed"`
}

func (s *Server) compactWorkflows(ctx context.Context, _ *mcp.CallToolRequest, input WorkflowCompactInput) (*mcp.CallToolResult, WorkflowCompactOutput, error) {
	removed, err := s.workflows.Compact(input.Limit)
	if err != nil {
		return nil, WorkflowCompactOutput{}, err
	}
	return nil, WorkflowCompactOutput{Removed: removed}, nil
}

func (s *Server) readWorkflowList(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	_ = req
	list := s.workflows.List()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "workflow://list",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func (s *Server) readWorkflow(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if req == nil || req.Params == nil {
		return nil, errors.New("missing resource params")
	}
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow URI: %w", err)
	}
	if u.Scheme != "workflow" {
		return nil, fmt.Errorf("unsupported workflow URI: %s", req.Params.URI)
	}
	id := strings.TrimPrefix(u.Path, "/")
	if id == "" {
		id = u.Host
	}
	if id == "" || id == "list" {
		return s.readWorkflowList(ctx, req)
	}
	w, ok := s.workflows.Get(id)
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}
