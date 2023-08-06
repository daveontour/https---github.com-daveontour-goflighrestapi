package globals

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	easy "github.com/t-tomalak/logrus-easy-formatter"
	"gopkg.in/natefinch/lumberjack.v2"

	"flightresourcerestapi/models"
)

var RepoList []models.Repository
var Wg sync.WaitGroup

var MapMutex = &sync.RWMutex{}

// var serviceConfig ServiceConfig
var IsDebug bool = false

var Logger = logrus.New()
var RequestLogger = logrus.New()
var MetricsLogger = logrus.New()

var ConfigViper = viper.New()
var UserViper = viper.New()
var AirportsViper = viper.New()

const UpdateAction = "UPDATE"
const CreateAction = "CREATE"
const DeleteAction = "DELETE"
const StatusAction = "STATUS"

var RepositoryUpdateChannel = make(chan int)
var FlightUpdatedChannel = make(chan models.Flight)
var FlightCreatedChannel = make(chan models.Flight)
var FlightDeletedChannel = make(chan models.Flight)
var FlightsInitChannel = make(chan int)

var SchedulerMap = make(map[string]*gocron.Scheduler)
var RefreshSchedulerMap = make(map[string]*gocron.Scheduler)

var UserChangeSubscriptions []models.UserChangeSubscription
var UserChangeSubscriptionsMutex = &sync.RWMutex{}

var ReservedParameters = []string{"airport", "airline", "al", "from", "to", "direction", "d", "route", "r", "sort"}

func InitGlobals() {

	exe, err0 := os.Executable()
	if err0 != nil {
		panic(err0)
	}

	exPath := filepath.Dir(exe)

	ConfigViper.SetConfigName("service") // name of config file (without extension)
	ConfigViper.SetConfigType("json")    // REQUIRED if the config file does not have the extension in the name
	ConfigViper.AddConfigPath(".")       // optionally look for config in the working directory
	ConfigViper.AddConfigPath(exPath)
	if err := ConfigViper.ReadInConfig(); err != nil {
		Logger.Fatal("Could Not Read service.json config file")
	}

	AirportsViper.SetConfigName("airports")
	AirportsViper.SetConfigType("json")
	AirportsViper.AddConfigPath(".") // optionally look for config in the working directory
	AirportsViper.AddConfigPath(exPath)
	if err := AirportsViper.ReadInConfig(); err != nil {
		Logger.Fatal("Could Not Read airports.json config file")
	}

	UserViper.SetConfigName("users")
	UserViper.SetConfigType("json")
	UserViper.AddConfigPath(".") // optionally look for config in the working directory
	UserViper.AddConfigPath(exPath)
	if err := UserViper.ReadInConfig(); err != nil {
		Logger.Fatal("Could Not Read users.json config file")
	}
	UserViper.OnConfigChange(func(e fsnotify.Event) {
		Logger.Warn("User Config File Changed. Re-reading it")
		if err := UserViper.ReadInConfig(); err != nil {
			Logger.Fatal("Could Not Read users.json config file")
		}
	})
	UserViper.WatchConfig()

	//serviceConfig = getServiceConfig()
	IsDebug = ConfigViper.GetBool("DebugService")

	initLogging()

	if ConfigViper.GetBool("EnableMetrics") {
		MetricsLogger.SetLevel(logrus.InfoLevel)
	} else {
		MetricsLogger.SetLevel(logrus.ErrorLevel)
	}

	Logger.SetLevel(logrus.InfoLevel)
	RequestLogger.SetLevel(logrus.InfoLevel)

	if IsDebug {
		Logger.SetLevel(logrus.DebugLevel)
	}

}

func initLogging() {
	Logger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if ConfigViper.GetString("LogFile") != "" {
		Logger.SetOutput(&lumberjack.Logger{
			Filename:   ConfigViper.GetString("LogFile"),
			MaxSize:    ConfigViper.GetInt("MaxLogFileSizeInM"), // megabytes
			MaxBackups: ConfigViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
	RequestLogger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if ConfigViper.GetString("RequestLogFile") != "" {
		RequestLogger.SetOutput(&lumberjack.Logger{
			Filename:   ConfigViper.GetString("RequestLogFile"),
			MaxSize:    ConfigViper.GetInt("MaxLogFileSizeInMB"), // megabytes
			MaxBackups: ConfigViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
	MetricsLogger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05.000000",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if ConfigViper.GetString("MetricsLogFile") != "" {
		MetricsLogger.SetOutput(&lumberjack.Logger{
			Filename:   ConfigViper.GetString("MetricsLogFile"),
			MaxSize:    ConfigViper.GetInt("MaxLogFileSizeInMB"), // megabytes
			MaxBackups: ConfigViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
}
