package puush

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var uploadFileName = flag.String("upload-file-name", "", "Path to file used for upload testing")

func TestUpload(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping upload test; provide -auth-email, -auth-password, and -auth-server-url")
	}
	if *uploadFileName == "" {
		t.Skip("skipping upload test; provide -upload-file-name")
	}

	file, err := os.Open(*uploadFileName)
	if err != nil {
		t.Fatalf("failed to open file for upload test: %v", err)
	}
	defer file.Close()

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	url, err := client.Upload(file, filepath.Base(*uploadFileName))
	if err != nil {
		t.Fatalf("Upload() returned error: %v", err)
	}
	if url == "" {
		t.Fatal("Upload() returned empty URL")
	}

	t.Logf("Upload successful: %s", url)
}

func TestUploadWithProgress(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping upload test; provide -auth-email, -auth-password, and -auth-server-url")
	}
	if *uploadFileName == "" {
		t.Skip("skipping upload test; provide -upload-file-name")
	}

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	var maxProgress float64

	progressReader, err := NewProgressReaderFromFile(*uploadFileName, func(percentage float64) {
		maxProgress = percentage
	})
	if err != nil {
		t.Fatalf("failed to create progress reader: %v", err)
	}
	defer progressReader.Close()

	url, err := client.Upload(progressReader, filepath.Base(*uploadFileName))
	if err != nil {
		t.Fatalf("Upload() returned error: %v", err)
	}
	if url == "" {
		t.Fatal("Upload() returned empty URL")
	}

	if maxProgress == 0 {
		t.Fatal("Upload progress did not track correctly")
	}
	t.Logf("Upload successful: %s (Max progress tracked: %.2f%%)", url, maxProgress)
}
