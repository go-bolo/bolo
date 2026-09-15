package bolo_test

// Matriz MIG-01..MIG-07 (plano 2: migrations).
//
// Fixtures: App proprio por teste com SQLite em arquivo dentro de t.TempDir
// (fechado em t.Cleanup); tabela de bookkeeping portatil criada pela fixture
// para testes de persistencia (sem DEFAULT NOW(), timestamps explicitos nos
// modelos); plugin fake com contadores de callbacks e behavior configuravel.
// Os testes entram na suite padrao mesmo vermelhos: sem build tags, sem t.Skip.
//
// Pre-condicao MIG-01: o DDL de producao usa DEFAULT NOW(), incompativel com
// SQLite (erro de parse, mesmo com IF NOT EXISTS e tabela pre-existente).
// Enquanto o DDL nao for portavel, os testes de runner (MIG-03..06) falham na
// pre-condicao com a dependencia registrada, sem gerar falso diagnostico.

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	bolo "github.com/go-bolo/bolo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

// DDL portatil da fixture: sem DEFAULT NOW() e com timestamps explicitos.
const migrationTestPortableDDL = `CREATE TABLE IF NOT EXISTS bolo_migrations (
	plugin_name varchar(200) NOT NULL,
	version INT NULL,
	last_upgrade_name varchar(255) NULL,
	installed bool DEFAULT false NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL,
	last_error TEXT NULL,
	CONSTRAINT plugin_name PRIMARY KEY (plugin_name)
)`

// App de teste com SQLite isolado em arquivo temporario; DB fechado no cleanup.
func newIsolatedSQLiteApp(t *testing.T) bolo.App {
	t.Helper()

	dbFile := filepath.Join(t.TempDir(), "bolo-migrations-test.db")
	gormDB, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: gorm_logger.Default.LogMode(gorm_logger.Silent),
	})
	require.NoError(t, err, "fixture: deveria abrir SQLite isolado em %s", dbFile)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err, "fixture: deveria expor o *sql.DB subjacente")
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	app := bolo.NewApp(&bolo.AppOptions{})
	require.NoError(t, app.SetDB(gormDB), "fixture: deveria registrar o DB no app")

	return app
}

// Pre-condicao dos testes de runner: setup com o DDL real de producao.
// Falhar aqui e bloqueio de MIG-01, nao repro do runner.
func requireMigrationEngineSetup(t *testing.T, app bolo.App) *bolo.MigrationEngine {
	t.Helper()

	engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})
	err := engine.SetupMigrationEngine()
	require.NoError(t, err,
		"pre-condicao bloqueada por MIG-01: o DDL de producao de bolo_migrations usa DEFAULT NOW(), "+
			"incompativel com SQLite; nenhum runner de migration executa no SQLite ate o DDL ser portavel")

	return engine
}

// Contadores de callbacks Up: execucoes por migration e ordem global.
// Behavior por migration permite erro, panic e retry.
type migrationTracker struct {
	mu        sync.Mutex
	calls     map[string]int
	execOrder []string
	behaviors map[string]func(app bolo.App) error
}

func newMigrationTracker() *migrationTracker {
	return &migrationTracker{
		calls:     map[string]int{},
		behaviors: map[string]func(app bolo.App) error{},
	}
}

func (tr *migrationTracker) setBehavior(name string, behavior func(app bolo.App) error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.behaviors[name] = behavior
}

// run registra contador/ordem e aplica o behavior configurado (nil = sucesso).
func (tr *migrationTracker) run(name string) func(app bolo.App) error {
	return func(app bolo.App) error {
		tr.mu.Lock()
		tr.calls[name]++
		tr.execOrder = append(tr.execOrder, name)
		behavior := tr.behaviors[name]
		tr.mu.Unlock()

		if behavior == nil {
			return nil
		}
		return behavior(app)
	}
}

func (tr *migrationTracker) count(name string) int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.calls[name]
}

func (tr *migrationTracker) order() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	out := make([]string, len(tr.execOrder))
	copy(out, tr.execOrder)
	return out
}

func failingBehavior(message string) func(app bolo.App) error {
	return func(app bolo.App) error {
		return errors.New(message)
	}
}

func panickingBehavior(value string) func(app bolo.App) error {
	return func(app bolo.App) error {
		panic(value)
	}
}

// Plugin fake minimo: Init, GetName e GetMigrations.
type fakeMigrationsPlugin struct {
	name           string
	migrations     []*bolo.Migration
	initCallsCount int
}

func (p *fakeMigrationsPlugin) Init(app bolo.App) error {
	p.initCallsCount++
	return nil
}

func (p *fakeMigrationsPlugin) GetName() string {
	return p.name
}

func (p *fakeMigrationsPlugin) GetMigrations() []*bolo.Migration {
	return p.migrations
}

func getBookkeepingRow(t *testing.T, app bolo.App, pluginName string) (*bolo.MigrationModel, error) {
	t.Helper()

	saved := &bolo.MigrationModel{}
	err := app.GetDB().
		Where("plugin_name = ?", pluginName).
		First(saved).Error

	return saved, err
}

// MIG-01: SetupMigrationEngine em SQLite novo deve criar a tabela utilizavel e
// ser idempotente. O DDL real usa DEFAULT NOW(); o teste expor a incompatibilidade.
func TestMIG01_SetupMigrationEngine_CreatesTableAndIsIdempotent(t *testing.T) {
	app := newIsolatedSQLiteApp(t)
	engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})

	err := engine.SetupMigrationEngine()
	require.NoError(t, err,
		"MIG-01 falha conhecida esperada: o DDL de producao de bolo_migrations usa DEFAULT NOW(), "+
			"incompativel com SQLite; enquanto o DDL nao for portavel, o setup bloqueia "+
			"o runner de migrations inteiro no SQLite")

	var tableCount int64
	err = app.GetDB().Raw(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'bolo_migrations'`,
	).Scan(&tableCount).Error
	require.NoError(t, err, "pre-condicao: leitura do sqlite_master deveria funcionar")
	assert.EqualValues(t, 1, tableCount, "tabela bolo_migrations deveria existir apos o setup")

	// A tabela precisa ser utilizavel: insert e leitura com timestamps explicitos.
	now := time.Now()
	err = app.GetDB().Exec(
		`INSERT INTO bolo_migrations
			(plugin_name, version, last_upgrade_name, installed, created_at, updated_at, last_error)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"plugin_probe", 1, "probe_migration", false, now, now, "",
	).Error
	require.NoError(t, err, "tabela recem-criada deveria aceitar insert com timestamps explicitos")

	var probe bolo.MigrationModel
	err = app.GetDB().Where("plugin_name = ?", "plugin_probe").First(&probe).Error
	require.NoError(t, err, "deveria ler de volta o registro de sonda")
	assert.Equal(t, "probe_migration", probe.LastUpgradeName)

	// Segunda chamada e idempotente (CREATE TABLE IF NOT EXISTS):
	err = engine.SetupMigrationEngine()
	assert.NoError(t, err, "segunda chamada de SetupMigrationEngine deveria ser idempotente")
}

// MIG-02: FindAllMigrations, FindAllMigrationsByPlugin e Save sobre a tabela
// portatil da fixture: criar, atualizar sem duplicar e devolver dados por plugin.
func TestMIG02_MigrationModelPersistence(t *testing.T) {
	app := newIsolatedSQLiteApp(t)
	require.NoError(t, app.GetDB().Exec(migrationTestPortableDDL).Error,
		"pre-condicao: fixture deveria criar a tabela portatil de bookkeeping")

	engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})

	// Tabela nova: lista vazia, sem erro.
	migs, err := engine.FindAllMigrations()
	require.NoError(t, err)
	require.NotNil(t, migs)
	assert.Empty(t, migs, "bookkeeping novo deveria estar vazio")

	// Save cria o registro (timestamps explicitos, como exige a tabela portatil):
	now := time.Now()
	model := &bolo.MigrationModel{
		PluginName:      "plugin_bookkeeping",
		Version:         2,
		LastUpgradeName: "create_widgets",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, model.Save(app), "Save deveria criar o registro novo")

	// Save de novo atualiza sem duplicar:
	model.Version = 3
	model.LastUpgradeName = "create_widgets_v2"
	model.UpdatedAt = now.Add(2 * time.Second)
	require.NoError(t, model.Save(app), "Save de registro existente deveria atualizar")

	all, err := engine.FindAllMigrations()
	require.NoError(t, err)
	require.Len(t, all, 1, "update nao deveria duplicar o registro")
	assert.Equal(t, "plugin_bookkeeping", all[0].PluginName)
	assert.Equal(t, 3, all[0].Version)
	assert.Equal(t, "create_widgets_v2", all[0].LastUpgradeName)
	assert.Empty(t, all[0].LastError)

	byPlugin, err := engine.FindAllMigrationsByPlugin()
	require.NoError(t, err)
	require.Len(t, byPlugin, 1, "mapa deveria conter apenas o plugin persistido")
	saved, ok := byPlugin["plugin_bookkeeping"]
	require.True(t, ok, "mapa deveria ser indexado por plugin_name")
	require.NotNil(t, saved)
	assert.Equal(t, 3, saved.Version)
	assert.Equal(t, "create_widgets_v2", saved.LastUpgradeName)
}

// MIG-02: consulta e Save retornam erro em tabela ausente e DB fechado.
func TestMIG02_MigrationModelErrors(t *testing.T) {
	t.Run("tabela ausente deve retornar erro", func(t *testing.T) {
		app := newIsolatedSQLiteApp(t) // sem criar a tabela
		engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})

		_, err := engine.FindAllMigrations()
		require.Error(t, err, "consulta sem a tabela bolo_migrations deveria retornar erro")

		_, err = engine.FindAllMigrationsByPlugin()
		assert.Error(t, err, "ByPlugin deveria propagar o erro de FindAllMigrations")

		now := time.Now()
		model := &bolo.MigrationModel{PluginName: "plugin_x", CreatedAt: now, UpdatedAt: now}
		assert.Error(t, model.Save(app), "Save sem a tabela deveria retornar erro")
	})

	t.Run("db fechado deve retornar erro", func(t *testing.T) {
		app := newIsolatedSQLiteApp(t)
		require.NoError(t, app.GetDB().Exec(migrationTestPortableDDL).Error)

		sqlDB, err := app.GetDB().DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close(), "pre-condicao: fechar o banco")

		engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})

		_, err = engine.FindAllMigrations()
		require.Error(t, err, "consulta apos fechar o banco deveria retornar erro")

		now := time.Now()
		model := &bolo.MigrationModel{PluginName: "plugin_x", CreatedAt: now, UpdatedAt: now}
		assert.Error(t, model.Save(app), "Save apos fechar o banco deveria retornar erro")
	})
}

// MIG-03: Up executa em ordem, persiste versao/nome, rerun nao repete as ja
// aplicadas e uma nova migration executa apenas o incremento.
func TestMIG03_UpRunsInOrderPersistsAndIsIncremental(t *testing.T) {
	app := newIsolatedSQLiteApp(t)
	tr := newMigrationTracker()

	plugin := &fakeMigrationsPlugin{
		name: "plugin_incremental",
		migrations: []*bolo.Migration{
			{Name: "m1", Up: tr.run("m1")},
			{Name: "m2", Up: tr.run("m2")},
			{Name: "m3", Up: tr.run("m3")},
		},
	}
	app.RegisterPlugin(plugin)

	requireMigrationEngineSetup(t, app)

	require.NoError(t, bolo.Up(app), "primeiro Up deveria aplicar todas as migrations")

	assert.Equal(t, 1, tr.count("m1"), "m1 deveria executar exatamente uma vez")
	assert.Equal(t, 1, tr.count("m2"), "m2 deveria executar exatamente uma vez")
	assert.Equal(t, 1, tr.count("m3"), "m3 deveria executar exatamente uma vez")
	assert.Equal(t, []string{"m1", "m2", "m3"}, tr.order(), "migrations deveriam executar em ordem")

	saved, err := getBookkeepingRow(t, app, "plugin_incremental")
	require.NoError(t, err, "bookkeeping deveria ser persistido pelo runner")
	assert.Equal(t, 3, saved.Version, "versao deveria refletir a ultima migration aplicada")
	assert.Equal(t, "m3", saved.LastUpgradeName)
	assert.Empty(t, saved.LastError)

	// Rerun nao repete as ja aplicadas:
	require.NoError(t, bolo.Up(app), "rerun sem novidades deveria ser no-op")
	assert.Equal(t, 1, tr.count("m1"), "rerun nao deveria repetir m1")
	assert.Equal(t, 1, tr.count("m2"), "rerun nao deveria repetir m2")
	assert.Equal(t, 1, tr.count("m3"), "rerun nao deveria repetir m3")

	// Nova migration executa so o incremento:
	plugin.migrations = append(plugin.migrations, &bolo.Migration{Name: "m4", Up: tr.run("m4")})
	require.NoError(t, bolo.Up(app), "Up com nova migration deveria aplicar so o incremento")
	assert.Equal(t, 1, tr.count("m1"), "m1 nao deveria reexecutar")
	assert.Equal(t, 1, tr.count("m2"), "m2 nao deveria reexecutar")
	assert.Equal(t, 1, tr.count("m3"), "m3 nao deveria reexecutar")
	assert.Equal(t, 1, tr.count("m4"), "apenas a nova migration deveria executar")

	saved, err = getBookkeepingRow(t, app, "plugin_incremental")
	require.NoError(t, err)
	assert.Equal(t, 4, saved.Version)
	assert.Equal(t, "m4", saved.LastUpgradeName)
}

// MIG-04: erro em migration propaga pelo Up, nao executa as proximas, registra
// LastError sem avancar a versao; o retry retoma da falha sem repetir as ja
// aplicadas, avanca a versao e limpa LastError ao concluir.
func TestMIG04_UpPropagatesErrorWithoutAdvancingVersion(t *testing.T) {
	app := newIsolatedSQLiteApp(t)
	tr := newMigrationTracker()
	tr.setBehavior("m2", failingBehavior("boom on m2"))

	plugin := &fakeMigrationsPlugin{
		name: "plugin_com_erro",
		migrations: []*bolo.Migration{
			{Name: "m1", Up: tr.run("m1")},
			{Name: "m2", Up: tr.run("m2")},
			{Name: "m3", Up: tr.run("m3")},
		},
	}
	app.RegisterPlugin(plugin)

	requireMigrationEngineSetup(t, app)

	err := bolo.Up(app)
	require.Error(t, err, "Up deveria propagar o erro da migration")
	assert.Contains(t, err.Error(), "m2", "erro deveria identificar a migration que falhou")
	assert.Equal(t, 1, tr.count("m1"), "m1 deveria ter executado antes da falha")
	assert.Equal(t, 1, tr.count("m2"), "m2 deveria ter chegado ao callback e falhado")
	assert.Equal(t, 0, tr.count("m3"), "migration seguinte nao deveria executar apos falha")

	saved, readErr := getBookkeepingRow(t, app, "plugin_com_erro")
	require.NoError(t, readErr, "bookkeeping deveria registrar o estado da falha")
	assert.Equal(t, 1, saved.Version, "versao nao deveria avancar quando a migration falha")
	assert.Equal(t, "m2", saved.LastUpgradeName)
	assert.Contains(t, saved.LastError, "boom on m2", "LastError deveria registrar o motivo")

	// Retry com a falha resolvida:
	tr.setBehavior("m2", nil)
	require.NoError(t, bolo.Up(app), "retry deveria concluir as migrations pendentes")
	assert.Equal(t, 1, tr.count("m1"), "m1 nao deveria repetir no retry")
	assert.Equal(t, 2, tr.count("m2"), "m2 deveria reexecutar no retry")
	assert.Equal(t, 1, tr.count("m3"), "m3 deveria executar apos o retry resolver m2")

	saved, readErr = getBookkeepingRow(t, app, "plugin_com_erro")
	require.NoError(t, readErr)
	assert.Equal(t, 3, saved.Version, "versao deveria avancar ate a ultima migration aplicada")
	assert.Equal(t, "m3", saved.LastUpgradeName)
	assert.Empty(t, saved.LastError, "LastError deveria ser limpo apos o retry concluir")
}

// MIG-05: panic em migration tem o mesmo contrato de falha de MIG-04, sem falso
// sucesso; contadores comprovam a chegada ao callback. O retry com a falha
// resolvida retoma e conclui como em MIG-04.
func TestMIG05_PanicInMigrationIsReportedAsFailure(t *testing.T) {
	app := newIsolatedSQLiteApp(t)
	tr := newMigrationTracker()
	tr.setBehavior("m2", panickingBehavior("boom panic on m2"))

	plugin := &fakeMigrationsPlugin{
		name: "plugin_com_panic",
		migrations: []*bolo.Migration{
			{Name: "m1", Up: tr.run("m1")},
			{Name: "m2", Up: tr.run("m2")},
			{Name: "m3", Up: tr.run("m3")},
		},
	}
	app.RegisterPlugin(plugin)

	requireMigrationEngineSetup(t, app)

	err := bolo.Up(app)

	// Pre-condicao do repro: o callback que panica foi atingido.
	assert.Equal(t, 1, tr.count("m1"), "m1 deveria ter executado antes do panic")
	assert.Equal(t, 1, tr.count("m2"),
		"contadores deveriam comprovar a chegada ao callback que panica")

	// Contrato desejado (mesmo de MIG-04): falha reportada, sem falso sucesso.
	assert.Error(t, err, "Up nao deveria retornar nil quando uma migration panica")
	assert.Equal(t, 0, tr.count("m3"), "migration seguinte nao deveria executar apos panic")

	saved, readErr := getBookkeepingRow(t, app, "plugin_com_panic")
	if assert.NoError(t, readErr,
		"bookkeeping deveria registrar a falha em vez de nao persistir nada") {
		assert.Equal(t, 1, saved.Version, "versao nao deveria avancar quando a migration panica")
		assert.Contains(t, saved.LastError, "panic", "LastError deveria registrar o panic")
	}

	// Retry com a falha resolvida: mesmo contrato de MIG-04.
	tr.setBehavior("m2", nil)
	require.NoError(t, bolo.Up(app), "retry deveria concluir as migrations pendentes")
	assert.Equal(t, 1, tr.count("m1"), "m1 nao deveria repetir no retry")
	assert.Equal(t, 2, tr.count("m2"), "m2 deveria reexecutar no retry")
	assert.Equal(t, 1, tr.count("m3"), "m3 deveria executar apos o retry")

	saved, readErr = getBookkeepingRow(t, app, "plugin_com_panic")
	require.NoError(t, readErr)
	assert.Equal(t, 3, saved.Version, "versao deveria avancar ate a ultima migration aplicada")
	assert.Equal(t, "m3", saved.LastUpgradeName)
	assert.Empty(t, saved.LastError, "LastError deveria estar limpo apos a conclusao")
}

// MIG-06: plugin sem migrations nao afeta os demais; erro ao persistir o
// bookkeeping nao pode ser reportado como sucesso.
func TestMIG06_EmptyPluginDoesNotAffectOthersAndBookkeepingFailureIsReported(t *testing.T) {
	t.Run("plugin sem migrations nao afeta demais", func(t *testing.T) {
		app := newIsolatedSQLiteApp(t)
		tr := newMigrationTracker()

		app.RegisterPlugin(&fakeMigrationsPlugin{name: "plugin_vazio"})
		app.RegisterPlugin(&fakeMigrationsPlugin{
			name: "plugin_ok",
			migrations: []*bolo.Migration{
				{Name: "m1", Up: tr.run("m1")},
			},
		})

		requireMigrationEngineSetup(t, app)

		require.NoError(t, bolo.Up(app))
		assert.Equal(t, 1, tr.count("m1"),
			"plugin com migrations deveria rodar mesmo com plugin vazio antes dele")

		engine := bolo.NewMigrationEngine(&bolo.NewMigrationEngineOpts{App: app})
		byPlugin, err := engine.FindAllMigrationsByPlugin()
		require.NoError(t, err)
		assert.NotContains(t, byPlugin, "plugin_vazio",
			"plugin sem migrations nao deveria gerar bookkeeping")
		require.Contains(t, byPlugin, "plugin_ok")
		assert.Equal(t, 1, byPlugin["plugin_ok"].Version)
	})

	t.Run("erro ao persistir bookkeeping nao e sucesso", func(t *testing.T) {
		app := newIsolatedSQLiteApp(t)
		tr := newMigrationTracker()

		app.RegisterPlugin(&fakeMigrationsPlugin{
			name: "plugin_bookkeeping_quebrado",
			migrations: []*bolo.Migration{
				{Name: "m1", Up: tr.run("m1")},
			},
		})

		// Tabela quebrada de proposito: SetupMigrationEngine usa IF NOT EXISTS e nao
		// a repara; o Save do bookkeeping deve falhar. Hoje o parse do DDL com
		// DEFAULT NOW() falha antes de qualquer callback (dependencia de MIG-01).
		require.NoError(t, app.GetDB().Exec(
			`CREATE TABLE bolo_migrations (plugin_name varchar(200) PRIMARY KEY)`,
		).Error, "fixture: deveria criar tabela de bookkeeping quebrada")

		err := bolo.Up(app)

		// Pre-condicao do repro: a migration chegou ao callback. Sem isso a falha
		// observada e do DDL, nao do bookkeeping.
		if !assert.Equal(t, 1, tr.count("m1"),
			"pre-condicao bloqueada por MIG-01: o parse do DDL com DEFAULT NOW() falha no SQLite "+
				"antes de qualquer callback; com o DDL portavel, a migration deveria executar e a "+
				"falha observada deveria ser do Save do bookkeeping") {
			return
		}

		require.Error(t, err,
			"quando a migration executa mas o bookkeeping nao pode ser persistido, "+
				"Up deveria reportar erro em vez de sucesso")
	})
}

// MIG-07: Down retorna erro explicito enquanto nao implementado, em vez de nil.
func TestMIG07_DownReturnsExplicitErrorWhileNotImplemented(t *testing.T) {
	app := newIsolatedSQLiteApp(t)

	err := bolo.Down(app)
	assert.Error(t, err,
		"Down nao esta implementado e deveria sinalizar isso com erro explicito, "+
			"em vez de retornar nil (falso sucesso de rollback)")
}
