package bolo

import (
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/go-bolo/bolo/configuration"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var appInstance App

func init() {
	initDotEnvConfigSupport()
}

func Init(options *AppOptions) App {
	appInstance = NewApp(options)

	InitSanitizer()

	appInstance.RegisterPlugin(&Plugin{Name: "catu"})
	return appInstance
}

// Init doc env config with default development.env configuration file
// The configuration file pattern is: [environment].env
func initDotEnvConfigSupport() {
	env, _ := os.LookupEnv("GO_ENV")

	if env == "" {
		env = "development"
	}

	if _, err := os.Stat(env + ".env"); err == nil {
		godotenv.Load(env + ".env")
	}
}

func GetApp() App {
	return appInstance
}

func GetConfiguration() configuration.ConfigurationInterface {
	return appInstance.GetConfiguration()
}

func GetDefaultDatabaseConnection() *gorm.DB {
	return appInstance.GetDB()
}

// NewBotContext - Create a new request context for non http calls and testing
func NewBotContext(app App) (*RequestContext, error) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	c := app.GetRouter().NewContext(req, res)

	return NewRequestContext(&RequestContextOpts{App: app, EchoContext: c}), nil
}
