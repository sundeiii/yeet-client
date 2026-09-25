package puush

import (
	"flag"
	"testing"
)

// Usage example, since I always forget how to run the command:
// go test ./pkg/puush -run TestAuthentication \
//     -auth-email "you@example.com" \
//     -auth-password "your-password" \
//     -auth-server-url "https://puush.me"

var (
	authEmail     = flag.String("auth-email", "", "Email used for testing")
	authPassword  = flag.String("auth-password", "", "Password used for testing")
	authServerURL = flag.String("auth-server-url", "https://puush.me", "Base server URL used for testing")
)

func TestAuthentication(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping integration auth test; provide -auth-email and -auth-password")
	}

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	if client.Account == nil {
		t.Fatal("client.Account is nil after authentication")
	}

	if client.Account.Credentials == nil {
		t.Fatal("client.Account.Credentials is nil after authentication")
	}

	if !client.Account.Credentials.HasApiKey() {
		t.Fatal("expected API key to be present in account credentials after authentication")
	}
}
