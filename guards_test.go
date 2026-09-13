package media

import (
	"bytes"
	"net/http"
	"testing"
)

// The mutating routes carried no guard before v0.7, so anyone could upload to
// or delete from the storage backend anonymously. These lock that shut.

func TestMutatingRoutesRejectAnonymous(t *testing.T) {
	app, _ := newApp(t, nil)

	cases := []struct {
		name string
		req  *http.Request
	}{
		{"upload", uploadRequest(t, "avatar.png", pngBytes, "")},
		{"update", jsonRequest(http.MethodPut, "/media/"+testUserID, `{"name":"x"}`)},
		{"delete", httptestNewRequest(http.MethodDelete, "/media/"+testUserID, nil)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(tc.req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", resp.StatusCode)
			}
		})
	}
}

func TestMutatingRoutesRejectInsufficientRole(t *testing.T) {
	app, _ := newApp(t, stubIdentity(testUserID, "reader"))

	resp, err := app.Test(uploadRequest(t, "avatar.png", pngBytes, ""))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestReadRoutesStayPublic(t *testing.T) {
	app, _ := newApp(t, nil)

	resp, err := app.Test(httptestNewRequest(http.MethodGet, "/media", nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func jsonRequest(method, path, body string) *http.Request {
	req := httptestNewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// image/svg+xml is an XML document that executes script when a browser renders
// it inline, so it must not ride in on the "image/" family.
func TestDefaultMimeAllowlistExcludesSVG(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IsAllowedMime("image/svg+xml") {
		t.Error("image/svg+xml is allowed by the default MIME allowlist")
	}
	for _, safe := range []string{"image/png", "image/jpeg", "image/webp"} {
		if !cfg.IsAllowedMime(safe) {
			t.Errorf("%s should be allowed by default", safe)
		}
	}
}

func TestDefaultMimeAllowlistExcludesExecutables(t *testing.T) {
	cfg := DefaultConfig()
	for _, bad := range []string{
		"application/x-httpd-php",
		"text/x-php",
		"application/x-sh",
		"application/x-executable",
		"text/html",
	} {
		if cfg.IsAllowedMime(bad) {
			t.Errorf("%s is allowed by the default MIME allowlist", bad)
		}
	}
}
