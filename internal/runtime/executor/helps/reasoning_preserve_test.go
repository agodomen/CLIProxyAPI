package helps

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestPreserveReasoningContent_InjectsFromHistory(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"result","reasoning_content":"my reasoning","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"file content"},
		{"role":"assistant","content":"next","tool_calls":[{"id":"c2","type":"function","function":{"name":"write","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c2","content":"ok"}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.3.reasoning_content")
	if !rc.Exists() {
		t.Fatal("expected reasoning_content to be injected into second assistant")
	}
	if rc.String() != "my reasoning" {
		t.Fatalf("reasoning_content = %q, want %q", rc.String(), "my reasoning")
	}
}

func TestPreserveReasoningContent_KeepsExistingValue(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"assistant","content":"a","reasoning_content":"first reasoning","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"ok"},
		{"role":"assistant","content":"b","reasoning_content":"second reasoning","tool_calls":[{"id":"c2","type":"function","function":{"name":"y","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.2.reasoning_content")
	if rc.String() != "second reasoning" {
		t.Fatalf("reasoning_content = %q, want %q (should not overwrite)", rc.String(), "second reasoning")
	}
}

func TestPreserveReasoningContent_SkipsMessagesWithoutToolCalls(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"assistant","content":"thinking...","reasoning_content":"r1"},
		{"role":"assistant","content":"final answer"}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.1.reasoning_content")
	if rc.Exists() {
		t.Fatal("assistant without tool_calls should not get reasoning_content")
	}
}

func TestPreserveReasoningContent_NoChangeWhenNoHistoryReasoning(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"hi","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.1.reasoning_content")
	if rc.Exists() {
		t.Fatal("should not fabricate reasoning_content when none seen in history")
	}
}

func TestPreserveReasoningContent_EmptyDoesNotOverwritePriorValid(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"assistant","content":"thinking text","reasoning_content":"r1","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"ok"},
		{"role":"assistant","content":"","reasoning_content":""},
		{"role":"tool","tool_call_id":"c2","content":"ok"},
		{"role":"assistant","content":"has content text","tool_calls":[{"id":"c3","type":"function","function":{"name":"z","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.4.reasoning_content")
	if !rc.Exists() || rc.String() != "r1" {
		t.Fatalf("reasoning_content = %q, want %q (empty should not overwrite prior valid)", rc.String(), "r1")
	}
}

func TestPreserveReasoningContent_AllEmptyReasoningInjectsEmpty(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"assistant","content":"","reasoning_content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"ok"},
		{"role":"assistant","content":"","tool_calls":[{"id":"c2","type":"function","function":{"name":"y","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.2.reasoning_content")
	if !rc.Exists() || rc.String() != "" {
		t.Fatalf("reasoning_content = %q, want empty string", rc.String())
	}
}

func TestPreserveReasoningContent_MultipleRounds(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"do something"},
		{"role":"assistant","content":"","reasoning_content":"thinking round 1","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"data"},
		{"role":"assistant","content":"","reasoning_content":"thinking round 2","tool_calls":[{"id":"c2","type":"function","function":{"name":"write","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c2","content":"ok"},
		{"role":"assistant","content":"done","reasoning_content":"final thinking"},
		{"role":"user","content":"more"},
		{"role":"assistant","content":"","tool_calls":[{"id":"c3","type":"function","function":{"name":"read","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.7.reasoning_content")
	if !rc.Exists() || rc.String() != "final thinking" {
		t.Fatalf("messages.7.reasoning_content = %q, want %q", rc.String(), "final thinking")
	}
}

func TestPreserveReasoningContent_SkipsNonAssistantRoles(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"reply","reasoning_content":"real reasoning","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"ok"},
		{"role":"assistant","content":"next","tool_calls":[{"id":"c2","type":"function","function":{"name":"y","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.3.reasoning_content")
	if !rc.Exists() || rc.String() != "real reasoning" {
		t.Fatalf("messages.3.reasoning_content = %q, want %q", rc.String(), "real reasoning")
	}
	userRC := gjson.GetBytes(out, "messages.0.reasoning_content")
	if userRC.Exists() {
		t.Fatal("user message should not have reasoning_content")
	}
}

func TestPreserveReasoningContent_InvalidJSON(t *testing.T) {
	out := PreserveReasoningContent([]byte(`{invalid`))
	if string(out) != `{invalid` {
		t.Fatal("should return unchanged on invalid JSON")
	}
}

func TestPreserveReasoningContent_EmptyInputs(t *testing.T) {
	out := PreserveReasoningContent(nil)
	if out != nil {
		t.Fatal("should return nil on nil input")
	}

	out2 := PreserveReasoningContent([]byte(`{}`))
	if string(out2) != `{}` {
		t.Fatal("should return unchanged when no messages")
	}
}

func TestPreserveReasoningContent_EmptyReasoningPreservesPriorNonEmpty(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"assistant","content":"","reasoning_content":"valid reasoning","tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c1","content":"ok"},
		{"role":"assistant","content":"","reasoning_content":""},
		{"role":"tool","tool_call_id":"c2","content":"ok"},
		{"role":"assistant","content":"","tool_calls":[{"id":"c3","type":"function","function":{"name":"y","arguments":"{}"}}]}
	]}`)

	out := PreserveReasoningContent(body)
	rc := gjson.GetBytes(out, "messages.4.reasoning_content")
	if !rc.Exists() || rc.String() != "valid reasoning" {
		t.Fatalf("reasoning_content = %q, want %q", rc.String(), "valid reasoning")
	}
}
