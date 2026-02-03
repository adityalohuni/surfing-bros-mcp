package wsbrowser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/adityalohuni/mcp-server/internal/browser"
	"github.com/adityalohuni/mcp-server/internal/page"
	"github.com/adityalohuni/mcp-server/internal/protocol"
	"github.com/adityalohuni/mcp-server/internal/wsbridge"
)

type Options struct {
	Timeout time.Duration
}

type Client struct {
	bridge  *wsbridge.Bridge
	reducer *page.Reducer
	store   *page.Store
	timeout time.Duration
}

func NewClient(bridge *wsbridge.Bridge, reducer *page.Reducer, store *page.Store, opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	if reducer == nil {
		reducer = page.NewReducer(page.ReduceOptions{})
	}
	if store == nil {
		store = page.NewStore()
	}
	return &Client{
		bridge:  bridge,
		reducer: reducer,
		store:   store,
		timeout: timeout,
	}
}

func (c *Client) Click(ctx context.Context, selector string) (browser.ClickResult, error) {
	if selector == "" {
		return browser.ClickResult{}, errors.New("selector is required")
	}
	if err := c.sendAction(ctx, protocol.CommandClick, protocol.ClickPayload{Selector: selector}); err != nil {
		return browser.ClickResult{}, err
	}
	return browser.ClickResult{Status: "ok", Selector: selector}, nil
}

func (c *Client) Snapshot(ctx context.Context, opts browser.SnapshotOptions) (page.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(protocol.SnapshotPayload{
		IncludeHidden: opts.IncludeHidden,
		MaxElements:   opts.MaxElements,
		MaxText:       opts.MaxText,
		IncludeHTML:   opts.IncludeHTML,
		MaxHTML:       opts.MaxHTML,
		MaxHTMLTokens: opts.MaxHTMLTokens,
	})
	if err != nil {
		return page.Snapshot{}, err
	}

	resp, err := c.bridge.SendCommand(ctx, c.makeCommand(ctx, protocol.CommandSnapshot, payload))
	if err != nil {
		return page.Snapshot{}, err
	}
	if !resp.OK {
		return page.Snapshot{}, errors.New(resp.Error)
	}

	var data protocol.SnapshotData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return page.Snapshot{}, err
	}

	raw := page.RawPage{
		URL:      data.URL,
		Title:    data.Title,
		Text:     data.Text,
		HTML:     data.HTML,
		Elements: mapElements(data.Elements),
	}

	snapshot := c.reducer.Reduce(raw)
	if snapshot.ID == "" {
		snapshot.ID = c.store.Put(snapshot)
	}
	return snapshot, nil
}

func (c *Client) Scroll(ctx context.Context, opts browser.ScrollOptions) (browser.ScrollResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandScroll, protocol.ScrollPayload{
		DeltaX:   opts.DeltaX,
		DeltaY:   opts.DeltaY,
		Selector: opts.Selector,
		Behavior: opts.Behavior,
		Block:    opts.Block,
	})
	if err != nil {
		return browser.ScrollResult{}, err
	}
	var out browser.ScrollResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.ScrollResult{}, err
	}
	return out, nil
}

func (c *Client) Hover(ctx context.Context, selector string) (browser.HoverResult, error) {
	if selector == "" {
		return browser.HoverResult{}, errors.New("selector is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandHover, protocol.HoverPayload{Selector: selector})
	if err != nil {
		return browser.HoverResult{}, err
	}
	var out browser.HoverResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.HoverResult{}, err
	}
	return out, nil
}

func (c *Client) Type(ctx context.Context, selector string, text string, pressEnter bool) (browser.TypeResult, error) {
	if selector == "" {
		return browser.TypeResult{}, errors.New("selector is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandTypeText, protocol.TypePayload{
		Selector:   selector,
		Text:       text,
		PressEnter: pressEnter,
	})
	if err != nil {
		return browser.TypeResult{}, err
	}
	var out browser.TypeResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.TypeResult{}, err
	}
	return out, nil
}

func (c *Client) Enter(ctx context.Context, selector string, key string) (browser.EnterResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandEnter, protocol.EnterPayload{Selector: selector, Key: key})
	if err != nil {
		return browser.EnterResult{}, err
	}
	var out browser.EnterResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.EnterResult{}, err
	}
	return out, nil
}

func (c *Client) Back(ctx context.Context) (browser.HistoryResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandBack, struct{}{})
	if err != nil {
		return browser.HistoryResult{}, err
	}
	var out browser.HistoryResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.HistoryResult{}, err
	}
	return out, nil
}

func (c *Client) Forward(ctx context.Context) (browser.HistoryResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandForward, struct{}{})
	if err != nil {
		return browser.HistoryResult{}, err
	}
	var out browser.HistoryResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.HistoryResult{}, err
	}
	return out, nil
}

func (c *Client) WaitForSelector(ctx context.Context, selector string, timeoutMs int) (browser.WaitForSelectorResult, error) {
	if selector == "" {
		return browser.WaitForSelectorResult{}, errors.New("selector is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandWaitFor, protocol.WaitForSelectorPayload{Selector: selector, TimeoutMs: timeoutMs})
	if err != nil {
		return browser.WaitForSelectorResult{}, err
	}
	var out browser.WaitForSelectorResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.WaitForSelectorResult{}, err
	}
	return out, nil
}

func (c *Client) Find(ctx context.Context, text string, limit int, radius int, caseSensitive bool) (browser.FindResult, error) {
	if text == "" {
		return browser.FindResult{}, errors.New("text is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandFind, protocol.FindPayload{
		Text:          text,
		Limit:         limit,
		Radius:        radius,
		CaseSensitive: caseSensitive,
	})
	if err != nil {
		return browser.FindResult{}, err
	}
	var out browser.FindResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.FindResult{}, err
	}
	return out, nil
}

func (c *Client) Navigate(ctx context.Context, url string) (browser.NavigateResult, error) {
	if url == "" {
		return browser.NavigateResult{}, errors.New("url is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandNavigate, protocol.NavigatePayload{URL: url})
	if err != nil {
		return browser.NavigateResult{}, err
	}
	var out browser.NavigateResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.NavigateResult{}, err
	}
	return out, nil
}

func (c *Client) Select(ctx context.Context, opts browser.SelectOptions) (browser.SelectResult, error) {
	if opts.Selector == "" {
		return browser.SelectResult{}, errors.New("selector is required")
	}
	resp, err := c.sendActionWithData(ctx, protocol.CommandSelect, protocol.SelectPayload{
		Selector:  opts.Selector,
		Value:     opts.Value,
		Label:     opts.Label,
		Index:     opts.Index,
		Values:    opts.Values,
		Labels:    opts.Labels,
		Indices:   opts.Indices,
		MatchMode: opts.MatchMode,
		Toggle:    opts.Toggle,
	})
	if err != nil {
		return browser.SelectResult{}, err
	}
	var out browser.SelectResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.SelectResult{}, err
	}
	return out, nil
}

func (c *Client) Screenshot(ctx context.Context, opts browser.ScreenshotOptions) (browser.ScreenshotResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandScreenshot, protocol.ScreenshotPayload{
		Selector:  opts.Selector,
		Padding:   opts.Padding,
		Format:    opts.Format,
		Quality:   opts.Quality,
		MaxWidth:  opts.MaxWidth,
		MaxHeight: opts.MaxHeight,
	})
	if err != nil {
		return browser.ScreenshotResult{}, err
	}
	var out browser.ScreenshotResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.ScreenshotResult{}, err
	}
	return out, nil
}

func (c *Client) StartRecording(ctx context.Context) (browser.RecordingStateResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandStartRecording, struct{}{})
	if err != nil {
		return browser.RecordingStateResult{}, err
	}
	var out browser.RecordingStateResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.RecordingStateResult{}, err
	}
	return out, nil
}

func (c *Client) StopRecording(ctx context.Context) (browser.RecordingStateResult, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandStopRecording, struct{}{})
	if err != nil {
		return browser.RecordingStateResult{}, err
	}
	var out browser.RecordingStateResult
	if err := decodeResponse(resp, &out); err != nil {
		return browser.RecordingStateResult{}, err
	}
	return out, nil
}

func (c *Client) GetRecording(ctx context.Context) ([]browser.RecordedAction, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandGetRecording, struct{}{})
	if err != nil {
		return nil, err
	}
	var out []browser.RecordedAction
	if err := decodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListTabs(ctx context.Context) ([]browser.TabInfo, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandListTabs, struct{}{})
	if err != nil {
		return nil, err
	}
	var out []browser.TabInfo
	if err := decodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) OpenTab(ctx context.Context, opts browser.OpenTabOptions) (browser.TabInfo, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandOpenTab, protocol.OpenTabPayload{
		URL:    opts.URL,
		Active: opts.Active,
		Pinned: opts.Pinned,
	})
	if err != nil {
		return browser.TabInfo{}, err
	}
	var out browser.TabInfo
	if err := decodeResponse(resp, &out); err != nil {
		return browser.TabInfo{}, err
	}
	return out, nil
}

func (c *Client) CloseTab(ctx context.Context, tabID int) error {
	_, err := c.sendActionWithData(ctx, protocol.CommandCloseTab, protocol.CloseTabPayload{TabID: tabID})
	return err
}

func (c *Client) ClaimTab(ctx context.Context, opts browser.ClaimTabOptions) (browser.TabInfo, error) {
	resp, err := c.sendActionWithData(ctx, protocol.CommandClaimTab, protocol.ClaimTabPayload{
		TabID:         opts.TabID,
		Mode:          opts.Mode,
		RequireActive: opts.RequireActive,
	})
	if err != nil {
		return browser.TabInfo{}, err
	}
	var out browser.TabInfo
	if err := decodeResponse(resp, &out); err != nil {
		return browser.TabInfo{}, err
	}
	return out, nil
}

func (c *Client) ReleaseTab(ctx context.Context, tabID int) error {
	_, err := c.sendActionWithData(ctx, protocol.CommandReleaseTab, protocol.ReleaseTabPayload{TabID: tabID})
	return err
}

func (c *Client) SetTabSharing(ctx context.Context, tabID int, allowShared bool) error {
	_, err := c.sendActionWithData(ctx, protocol.CommandSetTabSharing, protocol.SetTabSharingPayload{
		TabID:       tabID,
		AllowShared: allowShared,
	})
	return err
}

func mapElements(in []protocol.Element) []page.Element {
	if len(in) == 0 {
		return nil
	}
	out := make([]page.Element, 0, len(in))
	for _, el := range in {
		out = append(out, page.Element{
			Tag:         el.Tag,
			Text:        el.Text,
			Selector:    el.Selector,
			Href:        el.Href,
			InputType:   el.InputType,
			Name:        el.Name,
			ID:          el.ID,
			ARIALabel:   el.ARIALabel,
			Title:       el.Title,
			Alt:         el.Alt,
			Value:       el.Value,
			Placeholder: el.Placeholder,
			Context:     el.Context,
		})
	}
	return out
}

func (c *Client) sendAction(ctx context.Context, cmdType protocol.CommandType, payload any) error {
	_, err := c.sendActionWithData(ctx, cmdType, payload)
	return err
}

func (c *Client) sendActionWithData(ctx context.Context, cmdType protocol.CommandType, payload any) (protocol.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	raw, err := json.Marshal(payload)
	if err != nil {
		return protocol.Response{}, err
	}
	resp, err := c.bridge.SendCommand(ctx, c.makeCommand(ctx, cmdType, raw))
	if err != nil {
		return protocol.Response{}, err
	}
	if !resp.OK {
		if resp.Error == "" && resp.ErrorCode == "" {
			return protocol.Response{}, errors.New("browser action failed")
		}
		if resp.Error == "" {
			return protocol.Response{}, fmt.Errorf("browser action failed (%s)", resp.ErrorCode)
		}
		if resp.ErrorCode != "" {
			return protocol.Response{}, fmt.Errorf("%s (%s)", resp.Error, resp.ErrorCode)
		}
		return protocol.Response{}, errors.New(resp.Error)
	}
	return resp, nil
}

func (c *Client) makeCommand(ctx context.Context, cmdType protocol.CommandType, payload json.RawMessage) protocol.Command {
	cmd := protocol.Command{
		ID:      uuid.New().String(),
		Type:    cmdType,
		Payload: payload,
	}
	if target, ok := browser.TargetFromContext(ctx); ok {
		cmd.SessionID = target.SessionID
		cmd.TabID = target.TabID
	}
	return cmd
}

func decodeResponse(resp protocol.Response, out any) error {
	if len(resp.Data) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}
