package content

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iximiuz/labctl/api"
	contentpkg "github.com/iximiuz/labctl/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFiles(t *testing.T) {
	// Setup test directory structure
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── file1.txt
	//   ├── file2.txt
	//   ├── .git/
	//   │   ├── config
	//   │   └── objects/
	//   │       └── test.obj
	//   └── subdir/
	//       ├── file3.txt
	//       └── .git/
	//           └── config

	files := map[string]string{
		"file1.txt":             "content1",
		"file2.txt":             "content2",
		".git/config":           "git config",
		".git/objects/test.obj": "test object",
		"subdir/file3.txt":      "content3",
		"subdir/.git/config":    "subdir git config",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Test listFiles
	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	// Convert results to relative paths for easier testing
	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// Expected files (only non-.git files)
	expected := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
	}

	// Sort both slices for consistent comparison
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)

	// Verify .git files are not included
	for _, path := range relPaths {
		assert.NotContains(t, path, ".git")
	}
}

func TestListDirs(t *testing.T) {
	// Setup test directory structure
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── .git/
	//   │   └── objects/
	//   ├── dir1/
	//   │   └── subdir/
	//   └── dir2/
	//       └── .git/

	dirs := []string{
		".git/objects",
		"dir1/subdir",
		"dir2/.git",
	}

	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, dir), 0755))
	}

	// Test listDirs
	result, err := listDirs(tmpDir)
	require.NoError(t, err)

	// Convert results to relative paths for easier testing
	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// Expected directories (only non-.git directories)
	expected := []string{
		"dir1",
		"dir1/subdir",
		"dir2",
	}

	// Sort both slices for consistent comparison
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)

	// Verify .git directories are not included
	for _, path := range relPaths {
		assert.NotContains(t, path, ".git")
	}
}

func TestListFilesEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := listFiles(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListDirsEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := listDirs(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListFilesIgnoreBackupFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file and a ~ temp file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md~"), []byte("unsaved content"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// assert: the temp file is not listed
	assert.Equal(t, []string{"index.md"}, relPaths)
}

func TestListFilesLabctlIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── .labctlignore
	//   ├── index.md
	//   ├── CLAUDE.md        <- ignored by exact name pattern
	//   ├── AGENTS.md        <- ignored by exact name pattern
	//   ├── notes.txt
	//   ├── .omc/
	//   │   └── config       <- ignored because .omc/ dir is ignored
	//   └── subdir/
	//       ├── CLAUDE.md    <- ignored by exact name (matches basename)
	//       └── readme.md

	ignoreContent := "# Claude-specific files\nCLAUDE.md\nAGENTS.md\n.omc/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("claude"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("agents"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("notes"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".omc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".omc/config"), []byte("omc config"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/CLAUDE.md"), []byte("claude"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/readme.md"), []byte("readme"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"notes.txt",
		"subdir/readme.md",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListFilesLabctlIgnoreGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()

	ignoreContent := "*.log\nbuild/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "debug.log"), []byte("log"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build/output.bin"), []byte("bin"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/main.go"), []byte("code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/trace.log"), []byte("log"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"src/main.go",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListFilesLabctlIgnoreCascading(t *testing.T) {
	// Root .labctlignore excludes CLAUDE.md everywhere.
	// subdir/.labctlignore additionally excludes *.log within subdir and below.
	tmpDir := t.TempDir()

	// Root structure
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte("CLAUDE.md\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.log"), []byte("log"), 0644)) // NOT ignored at root level

	// subdir with its own .labctlignore
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/.labctlignore"), []byte("*.log\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/main.go"), []byte("code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/debug.log"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/CLAUDE.md"), []byte("ignored"), 0644)) // root rule cascades

	// nested inside subdir — both rule sets should apply
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir/nested"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/notes.md"), []byte("notes"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/trace.log"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/CLAUDE.md"), []byte("ignored"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"root.log",
		"subdir/main.go",
		"subdir/nested/notes.md",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListDirsLabctlIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	ignoreContent := ".omc/\nbuild/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".omc"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src/lib"), 0755))

	result, err := listDirs(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{"src", "src/lib"}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListContentFilesLocalUsesCanonicalRemotePaths(t *testing.T) {
	tmpDir := t.TempDir()
	files := []string{
		"index.md",
		filepath.Join("__static__", "asset.txt"),
		filepath.Join("nested", "file.txt"),
	}
	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(file), 0644))
	}

	result, err := listContentFilesLocal(tmpDir)
	require.NoError(t, err)

	got := slices.Collect(maps.Keys(result))
	slices.Sort(got)
	assert.Equal(t, []string{
		"__static__/asset.txt",
		"index.md",
		"nested/file.txt",
	}, got)
	for _, file := range got {
		assert.NotContains(t, file, `\`)
	}
}

func TestPushReconcilesCanonicalAndMalformedRemotePaths(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "__static__"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "nested"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "__static__", "asset.txt"),
		[]byte("asset"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "nested", "file.md"),
		[]byte("# Markdown"),
		0644,
	))

	const malformedRemoteFile = `__static__\asset.txt`

	var (
		mu                   sync.Mutex
		uploadedBinaryFile   string
		uploadedMarkdownFile string
		deletedRemoteFile    string
		uploadedBinaryBody   string
	)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/content/v2/files":
			writeJSON(t, w, http.StatusOK, []api.ContentFile{{
				Path:   malformedRemoteFile,
				Digest: "malformed",
			}})

		case r.Method == http.MethodPut && r.URL.Path == "/content/files":
			var body struct {
				File string `json:"file"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode upload request: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			uploadedBinaryFile = body.File
			mu.Unlock()
			writeJSON(t, w, http.StatusOK, map[string]string{
				"uploadUrl": server.URL + "/upload",
			})

		case r.Method == http.MethodPut && r.URL.Path == "/content/markdown":
			var body struct {
				File string `json:"file"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode markdown request: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			uploadedMarkdownFile = body.File
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPut && r.URL.Path == "/upload":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read upload body: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			uploadedBinaryBody = string(data)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && r.URL.Path == "/content/files":
			mu.Lock()
			deletedRemoteFile = r.URL.Query().Get("file")
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cli := newContentTestCLI(server.URL)
	err := RunPushOnce(context.Background(), cli, PushConfig{
		Kind:  contentpkg.KindChallenge,
		Name:  "windows-paths",
		Dir:   tmpDir,
		Force: true,
	})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "__static__/asset.txt", uploadedBinaryFile)
	assert.Equal(t, "nested/file.md", uploadedMarkdownFile)
	assert.Equal(t, "asset", uploadedBinaryBody)
	assert.Equal(t, malformedRemoteFile, deletedRemoteFile)
	assert.False(t, strings.Contains(uploadedBinaryFile, `\`))
	assert.False(t, strings.Contains(uploadedMarkdownFile, `\`))
}

func TestPushWatchUsesCanonicalPathAfterNestedUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "__static__", "asset.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(localFile), 0755))
	require.NoError(t, os.WriteFile(localFile, []byte("first"), 0644))

	uploads := make(chan string, 10)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/content/v2/files":
			writeJSON(t, w, http.StatusOK, []api.ContentFile{})

		case r.Method == http.MethodPut && r.URL.Path == "/content/files":
			var body struct {
				File string `json:"file"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode upload request: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(t, w, http.StatusOK, map[string]string{
				"uploadUrl": server.URL + "/upload",
			})
			uploads <- body.File

		case r.Method == http.MethodPut && r.URL.Path == "/upload":
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunPushWatch(ctx, newContentTestCLI(server.URL), PushConfig{
			Kind:  contentpkg.KindChallenge,
			Name:  "windows-watch-paths",
			Dir:   tmpDir,
			Force: true,
		})
	}()

	assertCanonicalUpload := func(stage string) {
		t.Helper()
		select {
		case file := <-uploads:
			assert.Equal(t, "__static__/asset.txt", file, stage)
			assert.NotContains(t, file, `\`, stage)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s upload", stage)
		}
	}

	assertCanonicalUpload("initial")

	// The watcher is installed immediately after the initial reconciliation.
	time.Sleep(250 * time.Millisecond)
	require.NoError(t, os.WriteFile(localFile, []byte("updated"), 0644))
	assertCanonicalUpload("watch update")

	// Let the in-flight upload response complete before stopping the watcher.
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out stopping content push watcher")
	}
}

func TestListContentFilesLocalSkipsVanishedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("content"), 0644))

	// A dangling symlink is listed by the directory walk but fails to open with
	// ErrNotExist - the same failure mode as a short-lived editor tmp file that
	// disappears between the listing and the checksum computation.
	if err := os.Symlink(filepath.Join(tmpDir, "gone"), filepath.Join(tmpDir, "vanished.md.tmp.47383")); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}

	result, err := listContentFilesLocal(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, []string{"index.md"}, slices.Collect(maps.Keys(result)))
}

func TestListFilesToleratesVanishedDir(t *testing.T) {
	_, err := listFilesRecursive(filepath.Join(t.TempDir(), "gone"), nil)
	assert.NoError(t, err)

	_, err = listDirsRecursive(filepath.Join(t.TempDir(), "gone"), nil)
	assert.NoError(t, err)
}
