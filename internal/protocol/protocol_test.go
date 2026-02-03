package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCommandRoundTrip(t *testing.T) {
	payload, err := json.Marshal(ClickPayload{Selector: "#submit"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	cmd := Command{ID: "1", Type: CommandClick, Payload: payload}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	var got Command
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if !reflect.DeepEqual(cmd, got) {
		t.Fatalf("round trip mismatch: %#v != %#v", cmd, got)
	}
}
