package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// mkResp builds a minimal *http.Response for parseRecipesResponse to chew on.
func mkResp(status int, contentType, body string) *http.Response {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestParseRecipesResponse_ValidArray(t *testing.T) {
	resp := mkResp(http.StatusOK, "application/json",
		`[{"filename":"a.zip","name":"alpha"},{"filename":"b.zip","name":"beta"}]`)
	recipes, err := parseRecipesResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 2 {
		t.Fatalf("got %d recipes, want 2", len(recipes))
	}
	if recipes[0].Name != "alpha" || recipes[1].Name != "beta" {
		t.Errorf("decoded recipes wrong: %+v", recipes)
	}
}

func TestParseRecipesResponse_EmptyArray(t *testing.T) {
	resp := mkResp(http.StatusOK, "application/json", `[]`)
	recipes, err := parseRecipesResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 0 {
		t.Fatalf("got %d recipes, want 0", len(recipes))
	}
}

func TestParseRecipesResponse_404ErrorPage(t *testing.T) {
	// The exact condition from issue #8: a 404 whose plaintext body starts
	// with "404". Must NOT reach the JSON decoder; must report the HTTP
	// status without leaking a Go type name.
	resp := mkResp(http.StatusNotFound, "text/plain; charset=utf-8", "404 page not found")
	_, err := parseRecipesResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error should name the HTTP status; got: %v", err)
	}
	if strings.Contains(err.Error(), "api.Recipe") || strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error leaks Go internals: %v", err)
	}
}

func TestParseRecipesResponse_5xx(t *testing.T) {
	resp := mkResp(http.StatusBadGateway, "text/html", "<html>502 Bad Gateway</html>")
	_, err := parseRecipesResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a 502 response, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("error should name the HTTP status; got: %v", err)
	}
}

func TestParseRecipesResponse_200NonJSON(t *testing.T) {
	// A 200 with a non-JSON content type (e.g. an HTML maintenance page
	// served with a 200) is caught before the decoder runs.
	resp := mkResp(http.StatusOK, "text/html", "<html>maintenance</html>")
	_, err := parseRecipesResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a non-JSON 200, got nil")
	}
	if !strings.Contains(err.Error(), "non-JSON") {
		t.Errorf("error should mention the non-JSON response; got: %v", err)
	}
}

func TestParseRecipesResponse_200JSONButWrongShape(t *testing.T) {
	// A 200 with a JSON content type but a body that isn't a []Recipe
	// (e.g. an object) must fail with a clean message, not a decoder dump.
	resp := mkResp(http.StatusOK, "application/json", `{"error":"nope"}`)
	_, err := parseRecipesResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a wrong-shape JSON body, got nil")
	}
	if strings.Contains(err.Error(), "api.Recipe") || strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("error leaks Go internals: %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected response") {
		t.Errorf("error should be the friendly unexpected-response message; got: %v", err)
	}
}
