# SurfingBro MCP Server

<img src="https://raw.githubusercontent.com/adityalohuni/surfing-bros/master/icon.png" alt="SurfingBro icon" width="96" />

This is the MCP server that bridges the LLM to the browser extension over WebSocket.

Repository: https://github.com/adityalohuni/surfing-bros-mcp  
Parent: https://github.com/adityalohuni/surfing-bros

## Quick Start

```bash
cd /home/hage/project/SurfingBro/mcp
go run ./cmd/mcp
```

The server listens for WebSocket connections at `ws://localhost:9099/ws`.

## Requirements

- Go 1.21+

## Daemon Mode

If you use `mcpd`:

```bash
go run ./cmd/mcpd
```

This exposes HTTP/SSE endpoints and admin routes (see `mcp/cmd/mcpd` for flags/env).

## MCP Tools

- `browser.click`
- `browser.snapshot`
- `browser.scroll`
- `browser.hover`
- `browser.type`
- `browser.enter`
- `browser.back`
- `browser.forward`
- `browser.wait_for_selector`
- `browser.find`
- `browser.navigate`
- `browser.select`
- `browser.screenshot`
- `browser.start_recording`
- `browser.stop_recording`
- `browser.get_recording`
- `workflow.save`
- `workflow.compact`

## Tool Payloads

Each tool maps directly to a WebSocket command:

```json
{ "type": "action", "payload": { ... } }
```

### click
```json
{ "selector": "button.buy" }
```

### scroll
```json
{
  "deltaX": 0,
  "deltaY": 600,
  "selector": ".scroll-area",
  "behavior": "smooth",
  "block": "center"
}
```

### find
```json
{
  "text": "pricing",
  "limit": 25,
  "radius": 60,
  "caseSensitive": false
}
```

### hover
```json
{ "selector": ".menu-item" }
```

### type
```json
{
  "selector": "input[name='q']",
  "text": "surfing bro",
  "pressEnter": true
}
```

### enter
```json
{ "selector": "input[name='q']", "key": "Enter" }
```

### back / forward
```json
{}
```

### navigate
```json
{ "url": "https://example.com" }
```

### waitForSelector
```json
{ "selector": ".checkout", "timeoutMs": 8000 }
```

### snapshot
```json
{
  "includeHidden": false,
  "maxElements": 120,
  "maxText": 4000,
  "includeHTML": false,
  "maxHTML": 20000,
  "maxHTMLTokens": 2000
}
```

### select
```json
{
  "selector": "select#tags",
  "values": ["news", "finance"],
  "matchMode": "partial",
  "toggle": true
}
```

### screenshot
```json
{
  "selector": ".hero-card",
  "padding": 8,
  "format": "png",
  "maxWidth": 800,
  "maxHeight": 800
}
```

If `selector` is omitted for screenshot, the current viewport is captured.

### workflow.save
```json
{
  "name": "login_flow",
  "description": "Log into Acme",
  "steps": []
}
```

If `steps` is empty, the current recording is saved.

### workflow.compact
```json
{ "limit": 500 }
```

## Responses

Responses from the extension are forwarded verbatim and include:

```json
{ "id": "uuid", "ok": true, "data": { ... } }
```

Errors:

```json
{ "id": "uuid", "ok": false, "error": "message", "errorCode": "CODE" }
```

Common `errorCode` values:

- `NO_ACTIVE_TAB`
- `ELEMENT_NOT_FOUND`
- `INVALID_INPUT`
- `INVALID_TARGET`
- `OPTION_NOT_FOUND`
- `TIMEOUT`
- `NO_ACTIVE_ELEMENT`
- `UNSUPPORTED_COMMAND`
- `COMMAND_FAILED`
- `SCREENSHOT_FAILED`

## Workflow Persistence

Workflows are persisted to `mcp/workflows.json`.

You can enable automatic compaction by setting `WorkflowLimit` when creating the server:

```go
mcpserver.Options{
  WorkflowLimit: 500,
}
```
