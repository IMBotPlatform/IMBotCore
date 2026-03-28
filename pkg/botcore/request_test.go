package botcore

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRequestSnapshotSaveAttachmentsAppliesDownloadTransform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("cipher-payload"))
	}))
	defer server.Close()

	snapshot := RequestSnapshot{
		Attachments: []Attachment{
			{
				Type: AttachmentTypeFile,
				URL:  server.URL + "/file.bin",
				DownloadTransform: func(downloaded []byte) ([]byte, error) {
					return bytes.ToUpper(downloaded), nil
				},
			},
		},
	}

	results, err := snapshot.SaveAttachments(t.TempDir())
	if err != nil {
		t.Fatalf("save attachments: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("unexpected results length: %d", len(results))
	}

	saved, err := os.ReadFile(results[0].Path)
	if err != nil {
		t.Fatalf("read saved attachment: %v", err)
	}
	if string(saved) != "CIPHER-PAYLOAD" {
		t.Fatalf("unexpected saved content: %s", string(saved))
	}
}

func TestReferenceSaveAttachmentsUsesAttachmentData(t *testing.T) {
	ref := Reference{
		Type: "image",
		Attachments: []Attachment{
			{
				Type: AttachmentTypeImage,
				URL:  "https://example.com/reference.png",
				Data: []byte("plain-image"),
			},
		},
	}

	results, err := ref.SaveAttachments(t.TempDir())
	if err != nil {
		t.Fatalf("save reference attachments: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("unexpected results length: %d", len(results))
	}

	saved, err := os.ReadFile(results[0].Path)
	if err != nil {
		t.Fatalf("read saved attachment: %v", err)
	}
	if string(saved) != "plain-image" {
		t.Fatalf("unexpected saved content: %s", string(saved))
	}

	if filepath.Base(results[0].Path) != "reference.png" {
		t.Fatalf("unexpected saved filename: %s", filepath.Base(results[0].Path))
	}
}
