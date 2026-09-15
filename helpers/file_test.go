package helpers_test

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-bolo/bolo/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipEntry describes one entry of a generated test zip archive.
type zipEntry struct {
	name    string
	content string
	isDir   bool
}

// createZip builds a zip archive at path. Entries get explicit unix modes so
// extracted files are readable after Unzip (real world archives carry modes).
func createZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	for _, e := range entries {
		mode := fs.FileMode(0o644)
		if e.isDir {
			mode = fs.ModeDir | 0o755
		}
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		hdr.SetMode(mode)
		writer, err := w.CreateHeader(hdr)
		require.NoError(t, err)
		if !e.isDir {
			_, err = writer.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	require.NoError(t, f.Sync())
}

func readFileAt(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "fixture follow-up read must work")
	return string(content)
}

// ---- ZIP-01: valid archives ----

func TestUnzip(t *testing.T) {
	t.Run("ZIP-01/extracts files subdirectories and empty file with correct paths and contents", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "bundle.zip")
		dest := filepath.Join(tmp, "out")
		createZip(t, src, []zipEntry{
			{name: "hello.txt", content: "hello world"},
			{name: "empty.txt", content: ""},
			{name: "sub/nested.txt", content: "nested content"},
			{name: "subdir/", isDir: true},
		})

		files, err := helpers.Unzip(src, dest)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			filepath.Join(dest, "hello.txt"),
			filepath.Join(dest, "empty.txt"),
			filepath.Join(dest, "sub", "nested.txt"),
			filepath.Join(dest, "subdir"),
		}, files)

		assert.Equal(t, "hello world", readFileAt(t, filepath.Join(dest, "hello.txt")))
		assert.Equal(t, "nested content", readFileAt(t, filepath.Join(dest, "sub", "nested.txt")))

		emptyInfo, err := os.Stat(filepath.Join(dest, "empty.txt"))
		require.NoError(t, err)
		assert.Zero(t, emptyInfo.Size(), "empty file must be extracted empty")

		subdirInfo, err := os.Stat(filepath.Join(dest, "subdir"))
		require.NoError(t, err)
		assert.True(t, subdirInfo.IsDir(), "directory entries must be created as directories")
	})

	t.Run("ZIP-01/empty zip extracts nothing without error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "empty.zip")
		dest := filepath.Join(tmp, "out")
		createZip(t, src, nil)

		files, err := helpers.Unzip(src, dest)

		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("ZIP-01/corrupted archive returns an error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "broken.zip")
		require.NoError(t, os.WriteFile(src, []byte("definitely not a zip archive"), 0o600))
		dest := filepath.Join(tmp, "out")

		files, err := helpers.Unzip(src, dest)

		require.Error(t, err)
		assert.Empty(t, files)
	})

	t.Run("ZIP-01/missing source file returns an error", func(t *testing.T) {
		tmp := t.TempDir()
		dest := filepath.Join(tmp, "out")

		files, err := helpers.Unzip(filepath.Join(tmp, "does-not-exist.zip"), dest)

		require.Error(t, err)
		assert.Empty(t, files)
	})
}

// ---- ZIP-02: path traversal protection ----

func TestUnzipZipSlip(t *testing.T) {
	t.Run("ZIP-02/parent traversal entry fails and writes nothing outside dest", func(t *testing.T) {
		tmp := t.TempDir()
		sentinelPath := filepath.Join(tmp, "sentinel.txt")
		require.NoError(t, os.WriteFile(sentinelPath, []byte("sentinel-intact"), 0o600))

		src := filepath.Join(tmp, "evil.zip")
		dest := filepath.Join(tmp, "out")
		createZip(t, src, []zipEntry{{name: "../escape.txt", content: "escaped"}})

		files, err := helpers.Unzip(src, dest)

		require.Error(t, err)
		assert.NotContains(t, files, filepath.Join(tmp, "escape.txt"))
		assert.NoFileExists(t, filepath.Join(tmp, "escape.txt"), "no file may be written outside dest")
		assert.Equal(t, "sentinel-intact", readFileAt(t, sentinelPath), "external sentinel must stay intact")
	})

	t.Run("ZIP-02/nested subdirectory traversal fails and writes nothing outside dest", func(t *testing.T) {
		tmp := t.TempDir()
		sentinelPath := filepath.Join(tmp, "sentinel.txt")
		require.NoError(t, os.WriteFile(sentinelPath, []byte("sentinel-intact"), 0o600))

		src := filepath.Join(tmp, "evil.zip")
		dest := filepath.Join(tmp, "out")
		createZip(t, src, []zipEntry{{name: "sub/../../outside.txt", content: "escaped"}})

		files, err := helpers.Unzip(src, dest)

		require.Error(t, err)
		assert.NotContains(t, files, filepath.Join(tmp, "outside.txt"))
		assert.NoFileExists(t, filepath.Join(tmp, "outside.txt"), "no file may be written outside dest")
		assert.Equal(t, "sentinel-intact", readFileAt(t, sentinelPath), "external sentinel must stay intact")
	})

	t.Run("ZIP-02/deep valid subdirectory extraction still works after protection", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "ok.zip")
		dest := filepath.Join(tmp, "out")
		createZip(t, src, []zipEntry{{name: "a/b/c/leaf.txt", content: "leaf content"}})

		files, err := helpers.Unzip(src, dest)

		require.NoError(t, err)
		assert.Equal(t, []string{filepath.Join(dest, "a", "b", "c", "leaf.txt")}, files)
		assert.Equal(t, "leaf content", readFileAt(t, filepath.Join(dest, "a", "b", "c", "leaf.txt")))
	})
}
