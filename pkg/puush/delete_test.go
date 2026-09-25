package puush

import "testing"

func TestDeleteLatestItem(t *testing.T) {
	if *authEmail == "" || *authPassword == "" || *authServerURL == "" {
		t.Skip("skipping thumbnail test; provide -auth-email and -auth-password")
	}

	client := NewClientFromLogin(*authEmail, *authPassword)
	client.SetBaseURL(*authServerURL)

	if err := client.Authenticate(); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}

	history, err := client.History()
	if err != nil {
		t.Fatalf("History() returned error: %v", err)
	}

	if len(history) == 0 {
		t.Skip("skipping delete test; no items in history to delete")
	}

	latestItem := history[0]
	updatedHistory, err := client.Delete(latestItem.Id)
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	for _, item := range updatedHistory {
		if item.Id == latestItem.Id {
			t.Fatalf("Delete() did not remove the item with ID '%d' from history", latestItem.Id)
		}
	}
}
