package bolo

type Pluginer interface {
	Init(app App) error
	GetName() string
	GetMigrations() []*Migration
}
