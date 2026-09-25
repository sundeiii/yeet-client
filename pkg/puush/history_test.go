package puush

import "testing"

func TestHistory(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping thumbnail test; provide -auth-email and -auth-password")
	}

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	response, err := client.History()
	if err != nil {
		t.Fatalf("History() returned error: %v", err)
	}

	for _, item := range response {
		t.Log("History item:", item)
	}
}
