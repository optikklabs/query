package contracts

import (
	"encoding/json"
	"testing"
)

func TestSuccessEncodesNilSliceAsEmptyArray(t *testing.T) {
	var rows []string
	b, err := json.Marshal(Success(rows))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != "[]" {
		t.Fatalf("data = %s, want []", got.Data)
	}
}

func TestSuccessKeepsNonSliceData(t *testing.T) {
	type item struct{ N int }
	if got := Success(item{N: 1}).Data; got != (item{N: 1}) {
		t.Fatalf("data = %#v", got)
	}
	if got := Success(nil).Data; got != nil {
		t.Fatalf("nil data became %#v", got)
	}
}
