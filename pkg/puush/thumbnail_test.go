package puush

import (
	"io"
	"testing"
)

func TestThumbnail(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping thumbnail test; provide -auth-email and -auth-password")
	}

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	thumbnail, err := client.Thumbnail(1)
	if err != nil {
		t.Fatalf("Thumbnail() returned error: %v", err)
	}
	defer thumbnail.Close()

	content, err := io.ReadAll(thumbnail)
	if err != nil {
		t.Fatalf("failed to read thumbnail content: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("thumbnail content is empty")
	}

	t.Log("Thumbnail content length:", len(content))
}
