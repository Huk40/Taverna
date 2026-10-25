package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousAndOversizedInput(t *testing.T) {
	for _, tc := range []struct {
		body, typ string
		limit     int64
		status    int
	}{
		{`{"name":"ok"}`, "application/json", 100, 200},
		{`{"name":"ok"}`, "text/plain", 100, 415},
		{`{"name":"ok"} {}`, "application/json", 100, 400},
		{`{"other":1}`, "application/json", 100, 400},
		{`{"name":"too long"}`, "application/json", 8, 413},
		{`{"name":"ok"}` + strings.Repeat(" ", 100), "application/json", 30, 413},
		{"", "application/json", 100, 400},
	} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.typ)
		w := httptest.NewRecorder()
		var payload struct {
			Name string `json:"name"`
		}
		ok := Decode(w, r, &payload, tc.limit, false)
		if w.Code != tc.status || ok != (tc.status == 200) {
			t.Fatalf("%q: status=%d ok=%v", tc.body, w.Code, ok)
		}
	}
}

func TestPaginationBounds(t *testing.T) {
	for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1", "?offset=abc"} {
		w := httptest.NewRecorder()
		if _, ok := Pagination(w, httptest.NewRequest("GET", "/"+query, nil)); ok || w.Code != 400 {
			t.Fatal(query)
		}
	}
	if UUID("invalid") || !UUID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa") {
		t.Fatal("UUID validation")
	}
}
