package bolo_test

import (
	"os"
	"testing"
	"time"

	approvals "github.com/approvals/go-approval-tests"
	"github.com/approvals/go-approval-tests/reporters"
	bolo "github.com/go-bolo/bolo"
	"github.com/go-bolo/clock"
)

func TestMain(m *testing.M) {
	r := approvals.UseReporter(reporters.NewVSCodeReporter())
	defer r.Close()
	approvals.UseFolder("testdata/approvals")

	os.Exit(m.Run())
}

func GetTestApp() bolo.App {
	os.Setenv("TEMPLATE_FOLDER", "./testdata/mocks/themes")

	app := bolo.NewApp(&bolo.AppOptions{})
	app.SetTheme("site")

	c := clock.NewMock()
	t, _ := time.Parse("2006-01-02", "2023-07-16")
	c.Set(t)

	app.SetClock(c)

	return app
}
