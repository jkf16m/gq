package session

import "testing"

func TestParse(t *testing.T) {
	input := "u edit README.md\\nplease\\tthere\na I will inspect it.\nt \"call 1\" cmd arguments=pwd\\nnow\ntr \"call 1\" contents\\nline two\n"
	messages, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "edit README.md\nplease\tthere" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if len(messages[1].ToolCalls) != 1 || messages[1].ToolCalls[0].Arguments != "pwd\nnow" {
		t.Fatalf("unexpected tool call: %#v", messages[1])
	}
	if messages[2].Role != "tool" || messages[2].ToolCallID != "call 1" || messages[2].Content != "contents\nline two" {
		t.Fatalf("unexpected tool result: %#v", messages[2])
	}
}

func TestEscape(t *testing.T) {
	value := "line 1\nline 2\t\\raw"
	encoded := Escape(value)
	if encoded != `line 1\nline 2\t\\raw` {
		t.Fatalf("unexpected encoding: %q", encoded)
	}
	decoded, err := Unescape(encoded)
	if err != nil || decoded != value {
		t.Fatalf("round trip failed: %q, %v", decoded, err)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{`x message`, `u`, `tr id`, `t id`} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("Parse(%q) succeeded; want error", input)
		}
	}
}
