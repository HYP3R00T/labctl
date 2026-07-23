package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iximiuz/labctl/content"
)

func TestDownloadContentFileUsesURLPathSeparators(t *testing.T) {
	const (
		contentName = "restore-unavailable-status-page-3d777d80"
		contentFile = "__static__/WARNING.txt"
		fileBody    = "warning"
	)

	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if strings.Contains(r.RequestURI, `\`) {
			t.Errorf("request URI contains a Windows path separator: %q", r.RequestURI)
		}
		_, _ = io.WriteString(w, fileBody)
	}))
	t.Cleanup(server.Close)

	client := NewClient(ClientOptions{
		BaseURL:    server.URL,
		APIBaseURL: server.URL,
	})
	dest := filepath.Join(t.TempDir(), "__static__", "WARNING.txt")

	if err := client.DownloadContentFile(
		context.Background(),
		content.KindChallenge,
		contentName,
		contentFile,
		dest,
	); err != nil {
		t.Fatalf("DownloadContentFile() error = %v", err)
	}

	const wantPath = "/content/files/challenges/" + contentName + "/" + contentFile
	if requestPath != wantPath {
		t.Fatalf("request path = %q, want %q", requestPath, wantPath)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", dest, err)
	}
	if string(got) != fileBody {
		t.Fatalf("downloaded content = %q, want %q", got, fileBody)
	}
}
