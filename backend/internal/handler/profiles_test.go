package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

func newProfileHandler() *ProfileHandler {
	return NewProfileHandler(nil, nil, nil, &sync.WaitGroup{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func TestUpdateProfile_DisplayNameTooLong(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	name := strings.Repeat("x", 101)
	body := `{"display_name":"` + name + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for display_name > 100 chars, got %d", resp.StatusCode)
	}
	if code := parseErrorCode(t, resp); code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", code)
	}
}

func TestUpdateProfile_DisplayNameAtLimit(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	// Exactly 100 chars — valid (will fail at DB, but validation passes)
	name := strings.Repeat("x", 100)
	body := `{"display_name":"` + name + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	// Should NOT be 422 (validation passes; DB call with nil pool will 500)
	if resp.StatusCode == http.StatusUnprocessableEntity {
		t.Errorf("expected validation to pass for 100-char display_name, got 422")
	}
}

func TestUpdateProfile_TaglineTooLong(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	tagline := strings.Repeat("t", 151)
	body := `{"tagline":"` + tagline + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for tagline > 150 chars, got %d", resp.StatusCode)
	}
}

func TestUpdateProfile_BioTooLong(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	bio := strings.Repeat("b", 2001)
	body := `{"bio":"` + bio + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for bio > 2000 chars, got %d", resp.StatusCode)
	}
}

func TestUpdateProfile_AvatarURLTooLong(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	url := "https://" + strings.Repeat("a", 494)
	body := `{"avatar_url":"` + url + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for avatar_url > 500 chars, got %d", resp.StatusCode)
	}
}

func TestUpdateProfile_LocationTooLong(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me", h.UpdateProfile)

	location := strings.Repeat("l", 101)
	body := `{"location":"` + location + `"}`
	req, _ := http.NewRequest("PATCH", "/profiles/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for location > 100 chars, got %d", resp.StatusCode)
	}
}

func TestSetVisibility_InvalidValue(t *testing.T) {
	h := newProfileHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/profiles/me/visibility", h.SetVisibility)

	req, _ := http.NewRequest("PATCH", "/profiles/me/visibility",
		strings.NewReader(`{"visibility":"protected"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid visibility value, got %d", resp.StatusCode)
	}
}

func TestSetVisibility_ValidValues(t *testing.T) {
	for _, visibility := range []string{"public", "private"} {
		t.Run(visibility, func(t *testing.T) {
			h := newProfileHandler()
			app := newApp(testUserID)
			app.Use(injectUser(testUserID))
			app.Patch("/profiles/me/visibility", h.SetVisibility)

			body := `{"visibility":"` + visibility + `"}`
			req, _ := http.NewRequest("PATCH", "/profiles/me/visibility", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			// Validation passes (will fail at nil pool DB call), but NOT 422
			if resp.StatusCode == http.StatusUnprocessableEntity {
				t.Errorf("expected visibility=%q to pass validation", visibility)
			}
		})
	}
}
