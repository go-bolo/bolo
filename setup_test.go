package bolo_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	approvals "github.com/approvals/go-approval-tests"
	"github.com/approvals/go-approval-tests/reporters"
	bolo "github.com/go-bolo/bolo"
	"github.com/go-bolo/bolo/http_client"
	"github.com/go-bolo/clock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	r := approvals.UseReporter(reporters.NewQuietReporter())
	approvals.UseFolder("testdata/approvals")

	code := m.Run()
	r.Close()
	os.Exit(code)
}

func isolateTestBootstrap(t *testing.T, app bolo.App) {
	t.Helper()
	t.Setenv("DB_ENGINE", "sqlite")
	t.Setenv("DB_URI", filepath.Join(t.TempDir(), "test.sqlite"))
	client := http_client.HttpClient
	t.Cleanup(func() {
		http_client.HttpClient = client
		if app.GetDB() != nil {
			db, err := app.GetDB().DB()
			require.NoError(t, err)
			require.NoError(t, db.Close())
		}
	})
}

func GetTestApp(t *testing.T) bolo.App {
	t.Helper()
	t.Setenv("TEMPLATE_FOLDER", "./testdata/mocks/themes")

	app := bolo.NewApp(&bolo.AppOptions{})
	isolateTestBootstrap(t, app)
	app.SetTheme("site")

	c := clock.NewMock()
	now, err := time.Parse("2006-01-02", "2023-07-16")
	require.NoError(t, err)
	c.Set(now)

	app.SetClock(c)

	return app
}
