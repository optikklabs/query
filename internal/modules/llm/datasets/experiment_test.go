package datasets

import "testing"

func TestBuildMessagesFromInputString(t *testing.T) {
	msgs := buildMessages("be terse", []byte(`{"input":"hello"}`))
	if len(msgs) != 2 {
		t.Fatalf("want system+user, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" || msgs[1].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestBuildMessagesFromMessagesArray(t *testing.T) {
	msgs := buildMessages("", []byte(`{"messages":[{"role":"user","content":"hi"}]}`))
	if len(msgs) != 1 || msgs[0].Content != "hi" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestBuildMessagesRawFallback(t *testing.T) {
	msgs := buildMessages("", []byte(`"just text"`))
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestExactMatch(t *testing.T) {
	if s, has := exactMatch([]byte(`"yes"`), " yes "); !has || s != 1 {
		t.Fatalf("trimmed equal should score 1, got %v has=%v", s, has)
	}
	if s, has := exactMatch([]byte(`{"output":"a"}`), "b"); !has || s != 0 {
		t.Fatalf("mismatch should score 0, got %v has=%v", s, has)
	}
	if _, has := exactMatch(nil, "anything"); has {
		t.Fatal("absent expected output must be unscored")
	}
}
