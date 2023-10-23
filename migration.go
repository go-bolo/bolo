package bolo

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Migration struct {
	Name string
	Up   func(app App) error
	Down func(app App) error
}

type MigrationEngine struct {
	App App
}

func (m *MigrationEngine) SetupMigrationEngine() error {
	app := m.App
	db := app.GetDB()

	err := db.Exec(`CREATE TABLE IF NOT EXISTS bolo_migrations (
		plugin_name varchar(200) NOT NULL,
		version INT NULL,
		last_upgrade_name varchar(255) NULL,
		installed bool DEFAULT false NOT NULL,
		created_at datetime DEFAULT NOW() NOT NULL,
		updated_at datetime DEFAULT NOW() NOT NULL,
		last_error TEXT NULL,
		CONSTRAINT plugin_name PRIMARY KEY (plugin_name)
	)`).Error
	if err != nil {
		return err
	}

	return nil
}

func (m *MigrationEngine) FindAllMigrations() ([]*MigrationModel, error) {
	db := m.App.GetDB()
	migs := []*MigrationModel{}

	err := db.
		Limit(3000).
		Find(&migs).Error

	return migs, err
}

func (m *MigrationEngine) FindAllMigrationsByPlugin() (map[string]*MigrationModel, error) {
	d := map[string]*MigrationModel{}

	migs, err := m.FindAllMigrations()
	if err != nil {
		return d, err
	}

	for _, mi := range migs {
		d[mi.PluginName] = mi
	}

	return d, nil
}

func (m *MigrationEngine) GetPluginMigrations() ([]*Migration, error) {
	return []*Migration{}, nil
}

type NewMigrationEngineOpts struct {
	App App
}

func NewMigrationEngine(opts *NewMigrationEngineOpts) *MigrationEngine {
	return &MigrationEngine{
		App: opts.App,
	}
}

type MigrationModel struct {
	PluginName      string    `gorm:"column:plugin_name;primaryKey;type:varchar(200)"`
	Version         int       `gorm:"column:version"`
	LastUpgradeName string    `gorm:"column:last_upgrade_name"`
	CreatedAt       time.Time `gorm:"column:created_at;default:NOW();not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;default:NOW();not null"`
	LastError       string    `gorm:"column:last_error;type:TEXT"`
}

func (m *MigrationModel) TableName() string {
	return "bolo_migrations"
}

func (m *MigrationModel) Save(app App) error {
	db := app.GetDB()

	saved := MigrationModel{}

	findErr := db.
		Select("plugin_name").
		Where("plugin_name = ?", m.PluginName).
		First(&saved).Error
	if findErr != nil {
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
	}

	if saved.PluginName == "" {
		err := app.GetDB().Create(m).Error
		if err != nil {
			return err
		}
	} else {
		err := app.GetDB().Save(m).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func Up(app App) error {
	m := NewMigrationEngine(&NewMigrationEngineOpts{
		App: app,
	})
	err := m.SetupMigrationEngine()
	if err != nil {
		return err
	}

	plugins := app.GetPlugins()

	logrus.WithFields(logrus.Fields{
		"PluginCount": len(plugins),
	}).Info("Starting migrations")

	migrationsSaved, err := m.FindAllMigrationsByPlugin()
	if err != nil {
		return err
	}

	for _, plugin := range plugins {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{
					"pluginName": plugin.GetName(),
					"error":      r,
				}).Error("Recovered from a error on run migration migration")
			}
		}()

		migs := plugin.GetMigrations()

		logrus.WithFields(logrus.Fields{
			"PluginName":     plugin.GetName(),
			"migrationCount": len(migs),
		}).Debug("Running plugin migrations")

		if len(migs) == 0 {
			continue
		}

		var lastMigRan *Migration

		lastVersionRan := migrationsSaved[plugin.GetName()]
		// not installed, register it:
		if lastVersionRan == nil {
			lastVersionRan = &MigrationModel{
				PluginName:      plugin.GetName(),
				Version:         0,
				LastUpgradeName: migs[0].Name,
				UpdatedAt:       time.Now(),
				CreatedAt:       time.Now(),
			}
		}

		for pVersion, mig := range migs {
			v := pVersion + 1

			logrus.WithFields(logrus.Fields{
				"PluginName":      plugin.GetName(),
				"foundLastMigRan": lastMigRan != nil,
				"version":         v,
			}).Debug("Mig:")

			if lastVersionRan.Version == 0 || lastMigRan != nil {
				err := mig.Up(app)
				if err != nil {
					lastVersionRan.LastUpgradeName = mig.Name
					lastVersionRan.LastError = err.Error()
					err2 := lastVersionRan.Save(app)
					if err2 != nil {
						return fmt.Errorf("error on save lastVersionRan %s: %w : %w", mig.Name, err2, err)
					}
					return fmt.Errorf("error on run migration up %s: %w", mig.Name, err)
				}

				lastVersionRan.Version = v
				lastVersionRan.LastUpgradeName = mig.Name
				err = lastVersionRan.Save(app)
				if err != nil {
					return fmt.Errorf("error on save lastVersionRan %s: %w", mig.Name, err)
				}

				lastMigRan = mig

				if len(migs) < pVersion+1 {
					logrus.WithFields(logrus.Fields{
						"PluginName": plugin.GetName(),
						"version":    v,
					}).Info("Migration done")
				}
			} else if v == lastVersionRan.Version {
				lastMigRan = mig
				continue
			}
		}
	}

	logrus.Info("Migrations done")

	return nil
}

func Down(app App) error {
	logrus.Warn("TODO!")
	return nil
}
