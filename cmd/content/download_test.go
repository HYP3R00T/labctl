package content

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iximiuz/labctl/api"
	contentpkg "github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

const (
	testContentName = "restore-unavailable-status-page-3d777d80"
	testContentFile = "__static__/WARNING.txt"
	testContentBody = "warning"
	testContentURL  = "https://labs.iximiuz.com/challenges/" + testContentName
)

func TestCreateContentDownloadsNestedInitialFile(t *testing.T) {
	server := newContentDownloadTestServer(t, true, http.StatusOK)
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "challenge")
	cli := newContentTestCLI(server.URL)
	err := runCreateContent(context.Background(), cli, &createOptions{
		kind:   contentpkg.KindChallenge,
		name:   "restore-unavailable-status-page",
		noOpen: true,
		DirOptions: DirOptions{
			dir: dir,
		},
	})
	if err != nil {
		t.Fatalf("runCreateContent() error = %v", err)
	}

	assertDownloadedContentFile(t, dir)
}

func TestPullContentDownloadsNestedFile(t *testing.T) {
	server := newContentDownloadTestServer(t, false, http.StatusOK)
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "challenge")
	cli := newContentTestCLI(server.URL)
	err := runPullContent(context.Background(), cli, &pullOptions{
		kind:  contentpkg.KindChallenge,
		name:  testContentName,
		force: true,
		DirOptions: DirOptions{
			dir: dir,
		},
	})
	if err != nil {
		t.Fatalf("runPullContent() error = %v", err)
	}

	assertDownloadedContentFile(t, dir)
}

func TestCreateContentReportsRecoveryAfterDownloadFailure(t *testing.T) {
	server := newContentDownloadTestServer(t, true, http.StatusNotFound)
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "challenge")
	cli := newContentTestCLI(server.URL)
	err := runCreateContent(context.Background(), cli, &createOptions{
		kind:   contentpkg.KindChallenge,
		name:   "restore-unavailable-status-page",
		noOpen: true,
		DirOptions: DirOptions{
			dir: dir,
		},
	})
	if err == nil {
		t.Fatal("runCreateContent() error = nil, want an initial download error")
	}

	for _, want := range []string{
		"initial content download failed after successful remote creation",
		"Remote challenge created successfully",
		"Name: " + testContentName,
		"URL: " + testContentURL,
		"The remote content was not deleted",
		`labctl content pull challenge ` + testContentName + ` --dir "` + dir + `"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%s", want, err)
		}
	}
}

func TestCreateContentDistinguishesRemoteCreationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/author":
			writeJSON(t, w, http.StatusOK, api.Author{})
		case r.Method == http.MethodPost && r.URL.Path == "/challenges":
			http.Error(w, "creation rejected", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cli := newContentTestCLI(server.URL)
	err := runCreateContent(context.Background(), cli, &createOptions{
		kind:   contentpkg.KindChallenge,
		name:   "restore-unavailable-status-page",
		noOpen: true,
		DirOptions: DirOptions{
			dir: filepath.Join(t.TempDir(), "challenge"),
		},
	})
	if err == nil {
		t.Fatal("runCreateContent() error = nil, want a remote creation error")
	}
	if !strings.Contains(err.Error(), "remote challenge creation failed") {
		t.Fatalf("error = %q, want a remote creation failure", err)
	}
	if strings.Contains(err.Error(), "created successfully") {
		t.Fatalf("remote creation error incorrectly reports success: %v", err)
	}
}

func TestCreateContentReportsLocalPreparationFailure(t *testing.T) {
	server := newContentDownloadTestServer(t, true, http.StatusOK)
	defer server.Close()

	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	dir := filepath.Join(parentFile, "challenge")

	cli := newContentTestCLI(server.URL)
	err := runCreateContent(context.Background(), cli, &createOptions{
		kind:   contentpkg.KindChallenge,
		name:   "restore-unavailable-status-page",
		noOpen: true,
		DirOptions: DirOptions{
			dir: dir,
		},
	})
	if err == nil {
		t.Fatal("runCreateContent() error = nil, want a local preparation error")
	}
	if !strings.Contains(err.Error(), "local directory preparation failed after successful remote creation") {
		t.Fatalf("error = %q, want a local directory preparation failure", err)
	}
	if !strings.Contains(err.Error(), "Name: "+testContentName) {
		t.Fatalf("error does not report the created remote name: %v", err)
	}
}

func newContentDownloadTestServer(
	t *testing.T,
	allowCreate bool,
	downloadStatus int,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/author" && allowCreate:
			writeJSON(t, w, http.StatusOK, api.Author{})

		case r.Method == http.MethodPost && r.URL.Path == "/challenges" && allowCreate:
			writeJSON(t, w, http.StatusOK, api.Challenge{
				Name:    testContentName,
				PageURL: testContentURL,
			})

		case r.Method == http.MethodGet && r.URL.Path == "/challenges/"+testContentName:
			writeJSON(t, w, http.StatusOK, api.Challenge{
				Name:    testContentName,
				PageURL: testContentURL,
			})

		case r.Method == http.MethodGet && r.URL.Path == "/content/files":
			if got := r.URL.Query().Get("kind"); got != "challenge" {
				t.Errorf("kind query = %q, want challenge", got)
			}
			if got := r.URL.Query().Get("name"); got != testContentName {
				t.Errorf("name query = %q, want %q", got, testContentName)
			}
			writeJSON(t, w, http.StatusOK, []string{testContentFile})

		case r.Method == http.MethodGet &&
			r.URL.Path == "/content/files/challenges/"+testContentName+"/"+testContentFile:
			if strings.Contains(r.RequestURI, `\`) {
				t.Errorf("download URI contains a Windows path separator: %q", r.RequestURI)
			}
			w.WriteHeader(downloadStatus)
			if downloadStatus == http.StatusOK {
				_, _ = io.WriteString(w, testContentBody)
			}

		default:
			http.NotFound(w, r)
		}
	}))
}

func newContentTestCLI(serverURL string) labcli.CLI {
	cli := labcli.NewCLI(
		io.NopCloser(strings.NewReader("")),
		io.Discard,
		io.Discard,
		"test",
	)
	cli.SetClient(api.NewClient(api.ClientOptions{
		BaseURL:    serverURL,
		APIBaseURL: serverURL,
	}))
	return cli
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		t.Errorf("Encode() error = %v", err)
	}
}

func assertDownloadedContentFile(t *testing.T, dir string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, "__static__", "WARNING.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != testContentBody {
		t.Fatalf("downloaded content = %q, want %q", got, testContentBody)
	}
}
