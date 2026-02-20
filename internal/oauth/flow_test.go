package oauth

import "testing"

func TestParseOAuthURL(t *testing.T) {
	t.Run("accepts https url", func(t *testing.T) {
		if _, err := parseOAuthURL("https://example.com/oauth/authorize"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("accepts localhost http url", func(t *testing.T) {
		if _, err := parseOAuthURL("http://127.0.0.1:8080/authorize"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects javascript scheme", func(t *testing.T) {
		if _, err := parseOAuthURL("javascript:alert(1)"); err == nil {
			t.Fatal("expected error for javascript scheme")
		}
	})

	t.Run("rejects missing host", func(t *testing.T) {
		if _, err := parseOAuthURL("https:///authorize"); err == nil {
			t.Fatal("expected error for missing host")
		}
	})
}
