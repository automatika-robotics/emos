package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestParsePluginsResponse_ValidArray(t *testing.T) {
	resp := mkResp(http.StatusOK, "application/json",
		`[{"filename":"robot-plugin-example","name":"Example","vendor":"Automatika",`+
			`"entry_point":"myrobot_plugin:MyRobotPlugin","repo":"https://example.test/r",`+
			`"ref":"","description":"d","tags":["example"]}]`)
	plugins, err := parsePluginsResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}
	p := plugins[0]
	if p.Filename != "robot-plugin-example" || p.EntryPoint != "myrobot_plugin:MyRobotPlugin" {
		t.Errorf("decoded plugin wrong: %+v", p)
	}
	if len(p.Tags) != 1 || p.Tags[0] != "example" {
		t.Errorf("tags decoded wrong: %+v", p.Tags)
	}
}

func TestParsePluginsResponse_404(t *testing.T) {
	resp := mkResp(http.StatusNotFound, "text/plain; charset=utf-8", "404 page not found")
	_, err := parsePluginsResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error should name the HTTP status; got: %v", err)
	}
	if strings.Contains(err.Error(), "api.Plugin") || strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error leaks Go internals: %v", err)
	}
}

func TestParsePluginsResponse_200NonJSON(t *testing.T) {
	resp := mkResp(http.StatusOK, "text/html", "<html>maintenance</html>")
	_, err := parsePluginsResponse(resp)
	if err == nil {
		t.Fatal("expected an error for a non-JSON 200, got nil")
	}
	if !strings.Contains(err.Error(), "non-JSON") {
		t.Errorf("error should mention the non-JSON response; got: %v", err)
	}
}

var verifiedAt = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

const claimedAnswer = `{
  "valid": true, "plugin_slug": "emos-plugin-lite3", "plugin_name": "DeepRobotics Lite3",
  "client_name": "ACME Security GmbH", "serial_number": "L3-2026-00417", "tier": "pro",
  "is_claimed": true, "claimed_at": "2026-09-21T10:42:07.123456Z",
  "support_months": 6, "support_until": "2027-03-21"
}`

func TestParseVerifyResponse_ClaimedLicense(t *testing.T) {
	lic, err := parseVerifyResponse(mkResp(200, "application/json", claimedAnswer), "KEY-1", verifiedAt)
	if err != nil {
		t.Fatalf("parseVerifyResponse: %v", err)
	}
	if lic.Key != "KEY-1" || !lic.VerifiedAt.Equal(verifiedAt) {
		t.Errorf("key and verification time come from the caller, got %q at %v", lic.Key, lic.VerifiedAt)
	}
	if lic.PluginSlug != "emos-plugin-lite3" || lic.PluginName != "DeepRobotics Lite3" ||
		lic.ClientName != "ACME Security GmbH" || lic.SerialNumber != "L3-2026-00417" || lic.Tier != "pro" {
		t.Errorf("licence fields not carried over: %+v", lic)
	}
	if lic.ClaimedAt == nil || lic.ClaimedAt.Year() != 2026 {
		t.Errorf("activation date not carried over: %+v", lic)
	}
}

// A key is verified only once its holder has activated it on the support
// portal: that is where the licence agreement is accepted.
func TestParseVerifyResponse_UnclaimedKeyIsNotVerified(t *testing.T) {
	body := `{"valid": true, "plugin_slug": "emos-plugin-m20", "plugin_name": "DeepRobotics M20",
	  "client_name": "ACME", "serial_number": "M20-7", "tier": "pro", "is_claimed": false,
	  "claimed_at": null, "support_months": 6, "support_until": null}`
	lic, err := parseVerifyResponse(mkResp(200, "application/json", body), "KEY-2", verifiedAt)
	if !errors.Is(err, ErrLicenseNotClaimed) || lic != nil {
		t.Fatalf("unclaimed key = %+v, %v, want no licence and ErrLicenseNotClaimed", lic, err)
	}
	if !strings.Contains(err.Error(), "https://support.automatikarobotics.com") {
		t.Errorf("the error must say where to activate the licence, got %q", err)
	}
	// An answer that leaves is_claimed out counts as unclaimed too.
	_, err = parseVerifyResponse(mkResp(200, "application/json", `{"valid": true, "plugin_slug": "emos-plugin-m20"}`), "KEY-2", verifiedAt)
	if !errors.Is(err, ErrLicenseNotClaimed) {
		t.Errorf("answer without is_claimed = %v, want ErrLicenseNotClaimed", err)
	}
}

// A licence issued before tiers existed comes with nulls. It is still a
// verified licence.
func TestParseVerifyResponse_ClaimedLicenseWithNulls(t *testing.T) {
	body := `{"valid": true, "plugin_slug": "emos-plugin-m20", "plugin_name": "DeepRobotics M20",
	  "client_name": null, "serial_number": "M20-7", "tier": null, "is_claimed": true,
	  "claimed_at": "2026-09-21T10:42:07Z", "support_months": null, "support_until": null}`
	lic, err := parseVerifyResponse(mkResp(200, "application/json", body), "KEY-3", verifiedAt)
	if err != nil {
		t.Fatalf("parseVerifyResponse: %v", err)
	}
	if lic.PluginSlug != "emos-plugin-m20" || lic.ClaimedAt == nil ||
		lic.ClientName != "" || lic.Tier != "" {
		t.Errorf("unexpected licence: %+v", lic)
	}
}

func TestParseVerifyResponse_RefusedKey(t *testing.T) {
	_, err := parseVerifyResponse(mkResp(401, "application/json", `{"detail":"Invalid or inactive license key"}`), "K", verifiedAt)
	if !errors.Is(err, ErrInvalidLicense) {
		t.Errorf("401 = %v, want ErrInvalidLicense", err)
	}
	_, err = parseVerifyResponse(mkResp(200, "application/json", `{"valid": false}`), "K", verifiedAt)
	if !errors.Is(err, ErrInvalidLicense) {
		t.Errorf("valid false = %v, want ErrInvalidLicense", err)
	}
}

// Anything else is the portal failing to answer, never a verdict on the key.
func TestParseVerifyResponse_NoAnswerIsNotARefusal(t *testing.T) {
	for name, resp := range map[string]*http.Response{
		"rate limited":   mkResp(429, "application/json", `{"error":"Rate limit exceeded: 10 per 1 minute"}`),
		"server error":   mkResp(502, "text/html", "<html>Bad Gateway</html>"),
		"not json":       mkResp(200, "text/html", "<html>maintenance</html>"),
		"no robot named": mkResp(200, "application/json", `{"valid": true, "is_claimed": true}`),
	} {
		resp.Status = http.StatusText(resp.StatusCode)
		lic, err := parseVerifyResponse(resp, "K", verifiedAt)
		if err == nil || lic != nil {
			t.Errorf("%s: want an error and no licence, got %+v, %v", name, lic, err)
		}
		if errors.Is(err, ErrInvalidLicense) || errors.Is(err, ErrLicenseNotClaimed) {
			t.Errorf("%s: must not be reported as a verdict on the key: %v", name, err)
		}
	}
}

func TestParseRecipesResponse_CarriesVariants(t *testing.T) {
	body := `[{"filename": "vision_follower", "name": "Vision-Based Person Following", "description": "d", "tags": ["vision"],
	  "variants": [{"id": "generic", "robot": null, "sensors": []},
	               {"id": "emos-plugin-lite3+emos-plugin-hikvision", "robot": "emos-plugin-lite3", "sensors": ["emos-plugin-hikvision"]}]}]`
	recipes, err := parseRecipesResponse(mkResp(200, "application/json", body))
	if err != nil || len(recipes) != 1 || len(recipes[0].Variants) != 2 {
		t.Fatalf("recipes = %+v, %v", recipes, err)
	}
	g, robot := recipes[0].Variants[0], recipes[0].Variants[1]
	if g.ID != GenericVariant || g.Robot != "" || len(g.Sensors) != 0 {
		t.Errorf("generic variant = %+v", g)
	}
	if robot.Robot != "emos-plugin-lite3" || len(robot.Sensors) != 1 || robot.Sensors[0] != "emos-plugin-hikvision" {
		t.Errorf("robot variant = %+v", robot)
	}
}

func TestRecipeURLKeepsThePlusOfAVariantID(t *testing.T) {
	got := recipeURL("vision_follower", "emos-plugin-lite3+emos-plugin-hikvision")
	if !strings.HasSuffix(got, "/recipes/vision_follower/emos-plugin-lite3+emos-plugin-hikvision") {
		t.Errorf("recipeURL = %q", got)
	}
}

func TestCheckRecipeResponseTellsTheRefusalsApart(t *testing.T) {
	if err := checkRecipeResponse(mkResp(200, "application/zip", "PK")); err != nil {
		t.Errorf("200 = %v", err)
	}
	if err := checkRecipeResponse(mkResp(401, "application/json", `{"detail":"Invalid or inactive license key"}`)); !errors.Is(err, ErrInvalidLicense) {
		t.Errorf("401 = %v, want ErrInvalidLicense", err)
	}
	if err := checkRecipeResponse(mkResp(404, "application/json", `{"detail":"Recipe not found"}`)); !errors.Is(err, ErrNoSuchRecipe) {
		t.Errorf("404 = %v, want ErrNoSuchRecipe", err)
	}
	// The portal's own words reach the operator
	err := checkRecipeResponse(mkResp(403, "application/json", `{"detail":"This license is for the DeepRobotics M20, not the DeepRobotics Lite3"}`))
	var refused *RecipeRefusedError
	if !errors.As(err, &refused) || !strings.Contains(refused.Reason, "DeepRobotics M20") {
		t.Errorf("403 = %v, want the portal's reason", err)
	}
	if err := checkRecipeResponse(mkResp(403, "text/html", "<html>")); !errors.As(err, &refused) {
		t.Errorf("403 without a body = %v, want a RecipeRefusedError", err)
	}
	if err := checkRecipeResponse(mkResp(500, "text/html", "")); err == nil {
		t.Error("500 must be an error")
	}
}
