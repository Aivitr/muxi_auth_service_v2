package middleware

import "testing"

func TestRedactQuery(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no query", "/auth/api/user", "/auth/api/user"},
		{"token", "/auth/api/check_token?email=a@b.com&token=secret", "/auth/api/check_token?email=a@b.com&token=***"},
		{"token first", "/auth/api/check_token?token=secret", "/auth/api/check_token?token=***"},
		{"case insensitive key", "/x?TOKEN=secret", "/x?TOKEN=***"},
		{"safe params untouched", "/auth/api/oauth?grant_type=code&client_id=abc", "/auth/api/oauth?grant_type=code&client_id=abc"},
		{"valueless key", "/x?token", "/x?token"},
		{"empty value still redacted", "/x?token=", "/x?token=***"},
		{"suffix key not redacted", "/x?token_type=bearer", "/x?token_type=bearer"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := redactQuery(c.in); got != c.want {
				t.Fatalf("redactQuery(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
