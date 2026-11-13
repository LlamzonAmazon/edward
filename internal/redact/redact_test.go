package redact

import (
	"strings"
	"testing"
)

func TestBearerToken(t *testing.T) {
	in := `curl -H "Authorization: Bearer SECRET" https://api.github.com/user`
	out := String(in)
	if strings.Contains(out, "SECRET") {
		t.Fatalf("secret still present: %q", out)
	}
	if !strings.Contains(out, Mark) {
		t.Fatalf("missing %s: %q", Mark, out)
	}
}

func TestAPIKeyAndAKIA(t *testing.T) {
	in := "tool --api-key hunter2 --token=abc AKIAIOSFODNN7EXAMPLE rest"
	out := String(in)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "abc") || strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secret still present: %q", out)
	}
	if !strings.Contains(out, Mark) {
		t.Fatalf("missing mark: %q", out)
	}
}

func TestTokenShapes(t *testing.T) {
	cases := []struct{ in, secret string }{
		{"gh api -H x ghp_abcdefghijklmnopqrstuvwxyz0123", "ghp_abcdefghijklmnopqrstuvwxyz0123"},
		{"git clone https://github_pat_11ABCDEFG0abcdefghijklmnopqrstuv@github.com/a/b", "github_pat_11ABCDEFG0abcdefghijklmnopqrstuv"},
		{"node app.js sk-ant-api03-abcdefghijklmnopqrstuvwxyz", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"},
		{"python run.py sk-proj-abcdefghijklmnopqrstuvwx", "sk-proj-abcdefghijklmnopqrstuvwx"},
		{"bot xoxb-1234567890-abcdefghij", "xoxb-1234567890-abcdefghij"},
		{"client eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTYifQ.SflKxwRJSMeKKF2QT4", "eyJhbGciOiJIUzI1NiJ9"},
		{"psql postgres://admin:s3cr3t@db.local:5432/app", "s3cr3t"},
		{"mysql -u root -phunter22 appdb", "hunter22"},
		{"echo -----BEGIN RSA PRIVATE KEY-----", "BEGIN RSA PRIVATE KEY"},
	}
	for _, c := range cases {
		out := String(c.in)
		if strings.Contains(out, c.secret) {
			t.Errorf("secret left in %q", out)
		}
		if !strings.Contains(out, Mark) {
			t.Errorf("no mark in %q", out)
		}
	}
}

func TestNearMissesUnchanged(t *testing.T) {
	for _, in := range []string{
		"/usr/bin/ssh -p 2222 host",
		"grep -p pattern file",
		"https://github.com/user/repo",
		"postgres://db.local:5432/app",
		"ls -la /Users/me/.ssh",
		"node --max-old-space-size=4096 server.js",
		"sk-short",
		"/Applications/Slack.app/Contents/MacOS/Slack",
	} {
		if out := String(in); out != in {
			t.Errorf("changed %q to %q", in, out)
		}
	}
	if out := String("postgres://admin:s3cr3t@db.local/app"); !strings.Contains(out, "postgres://admin:") {
		t.Errorf("user name lost: %q", out)
	}
}
