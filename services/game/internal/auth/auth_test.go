package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var secret = []byte(strings.Repeat("k", 32))

func TestIssueVerify(t *testing.T) {
	now := time.Now()
	iss, err := NewIssuer(secret, 15*time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	p := Principal{UserID: uuid.New(), Role: "admin"}
	tok, err := iss.Issue(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.Verify(tok)
	if err != nil || got != p {
		t.Fatalf("verify: %v %+v", err, got)
	}
	// expired
	late, _ := NewIssuer(secret, time.Minute, func() time.Time { return now.Add(time.Hour) })
	if _, err := late.Verify(tok); err == nil {
		t.Fatal("expired token accepted")
	}
	// wrong key
	other, _ := NewIssuer([]byte(strings.Repeat("x", 32)), time.Minute, time.Now)
	if _, err := other.Verify(tok); err == nil {
		t.Fatal("foreign signature accepted")
	}
}

func TestRejectsShortSecretAndNoneAlg(t *testing.T) {
	if _, err := NewIssuer([]byte("short"), time.Minute, time.Now); err == nil {
		t.Fatal("short secret accepted")
	}
	iss, _ := NewIssuer(secret, time.Minute, time.Now)
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ4In0."
	if _, err := iss.Verify(none); err == nil {
		t.Fatal("alg=none accepted")
	}
}

func TestFlowSigner(t *testing.T) {
	f := NewFlowSigner(secret)
	now := time.Now()
	enc := f.Encode(FlowState{State: "s", Nonce: "n", Verifier: "v", ReturnTo: "/x", Expires: now.Add(time.Minute)})
	if s, err := f.Decode(enc, now); err != nil || s.State != "s" {
		t.Fatalf("decode: %v", err)
	}
	if _, err := f.Decode(enc, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired flow accepted")
	}
	if _, err := f.Decode(enc[:len(enc)-2]+"xx", now); err == nil {
		t.Fatal("tampered flow accepted")
	}
}

func TestSafeReturnPath(t *testing.T) {
	for in, want := range map[string]string{
		"/cultivate": "/cultivate", "": "/", "https://evil.com": "/", "//evil.com": "/", "/\\evil.com": "/", "/a\r\nSet-Cookie:x": "/",
	} {
		if got := SafeReturnPath(in); got != want {
			t.Errorf("SafeReturnPath(%q) = %q", in, got)
		}
	}
}
