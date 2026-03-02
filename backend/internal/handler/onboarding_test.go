package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

func newOnboardingHandler() *OnboardingHandler {
	return NewOnboardingHandler(nil, nil, nil, nil, nil, &sync.WaitGroup{},
		slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

// validIkigaiBody builds a request body with all four Ikigai fields at least 10 chars each.
func validIkigaiBody() string {
	return `{
		"what_you_love":            "building great software products",
		"what_youre_good_at":       "backend engineering and system design",
		"what_world_needs":         "reliable and scalable infrastructure",
		"what_you_can_be_paid_for": "consulting and software development"
	}`
}

func TestSaveIkigai_AllFieldsRequired(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/ikigai", h.SaveIkigai)

	cases := []struct {
		name string
		body string
	}{
		{"missing what_you_love", `{"what_youre_good_at":"ten chars ok","what_world_needs":"ten chars ok","what_you_can_be_paid_for":"ten chars ok"}`},
		{"missing what_youre_good_at", `{"what_you_love":"ten chars ok","what_world_needs":"ten chars ok","what_you_can_be_paid_for":"ten chars ok"}`},
		{"missing what_world_needs", `{"what_you_love":"ten chars ok","what_youre_good_at":"ten chars ok","what_you_can_be_paid_for":"ten chars ok"}`},
		{"missing what_you_can_be_paid_for", `{"what_you_love":"ten chars ok","what_youre_good_at":"ten chars ok","what_world_needs":"ten chars ok"}`},
		{"all empty", `{"what_you_love":"","what_youre_good_at":"","what_world_needs":"","what_you_can_be_paid_for":""}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/onboarding/ikigai", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("expected 422, got %d", resp.StatusCode)
			}
			if code := parseErrorCode(t, resp); code != "VALIDATION_ERROR" {
				t.Errorf("expected VALIDATION_ERROR, got %s", code)
			}
		})
	}
}

func TestSaveIkigai_FieldTooShort(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/ikigai", h.SaveIkigai)

	// "short" is only 5 chars — below the 10-char minimum
	body := `{
		"what_you_love":            "short",
		"what_youre_good_at":       "backend engineering long enough",
		"what_world_needs":         "reliable software products",
		"what_you_can_be_paid_for": "consulting and coding"
	}`
	req, _ := http.NewRequest("POST", "/onboarding/ikigai", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for too-short field, got %d", resp.StatusCode)
	}
}

func TestSaveIkigai_FieldTooLong(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/ikigai", h.SaveIkigai)

	longField := strings.Repeat("a", 1001)
	body := `{
		"what_you_love":            "` + longField + `",
		"what_youre_good_at":       "backend engineering long enough",
		"what_world_needs":         "reliable software products",
		"what_you_can_be_paid_for": "consulting and coding"
	}`
	req, _ := http.NewRequest("POST", "/onboarding/ikigai", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for too-long field (>1000 chars), got %d", resp.StatusCode)
	}
}

func TestSaveIkigai_FieldAtMinLength(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/ikigai", h.SaveIkigai)

	// Exactly 10 chars — should pass validation (fail at nil DB, but not 422)
	body := `{
		"what_you_love":            "1234567890",
		"what_youre_good_at":       "0987654321",
		"what_world_needs":         "abcdefghij",
		"what_you_can_be_paid_for": "jihgfedcba"
	}`
	req, _ := http.NewRequest("POST", "/onboarding/ikigai", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		t.Errorf("expected validation to pass at min length (10 chars), got 422")
	}
}

func TestSaveIkigai_FieldAtMaxLength(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/ikigai", h.SaveIkigai)

	// Exactly 1000 chars — should pass validation
	field1000 := strings.Repeat("x", 1000)
	body := `{
		"what_you_love":            "` + field1000 + `",
		"what_youre_good_at":       "backend engineering long enough",
		"what_world_needs":         "reliable software products",
		"what_you_can_be_paid_for": "consulting and coding"
	}`
	req, _ := http.NewRequest("POST", "/onboarding/ikigai", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		t.Errorf("expected validation to pass at max length (1000 chars), got 422")
	}
}

func TestImportCV_MissingURL(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/import-cv", h.ImportCV)

	req, _ := http.NewRequest("POST", "/onboarding/import-cv", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing URL, got %d", resp.StatusCode)
	}
}

func TestImportCV_EmptyURL(t *testing.T) {
	h := newOnboardingHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/onboarding/import-cv", h.ImportCV)

	req, _ := http.NewRequest("POST", "/onboarding/import-cv", strings.NewReader(`{"url":""}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty URL, got %d", resp.StatusCode)
	}
}
