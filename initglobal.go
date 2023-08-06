package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	easy "github.com/t-tomalak/logrus-easy-formatter"
	"gopkg.in/natefinch/lumberjack.v2"
)

var repoList []Repository
var wg sync.WaitGroup

var mapMutex = &sync.RWMutex{}

// var serviceConfig ServiceConfig
var isDebug bool = false

var logger = logrus.New()
var requestLogger = logrus.New()
var metricsLogger = logrus.New()

var configViper = viper.New()
var userViper = viper.New()
var airportsViper = viper.New()

const layout = "2006-01-02T15:04:05"

var loc *time.Location

const UpdateAction = "UPDATE"
const CreateAction = "CREATE"
const DeleteAction = "DELETE"
const StatusAction = "STATUS"

var repositoryUpdateChannel = make(chan int)
var flightUpdatedChannel = make(chan Flight)
var flightCreatedChannel = make(chan Flight)
var flightDeletedChannel = make(chan Flight)
var flightsInitChannel = make(chan int)

var schedulerMap = make(map[string]*gocron.Scheduler)
var refreshSchedulerMap = make(map[string]*gocron.Scheduler)

var userChangeSubscriptions []UserChangeSubscription
var userChangeSubscriptionsMutex = &sync.RWMutex{}

var reservedParameters = []string{"airport", "airline", "al", "from", "to", "direction", "d", "route", "r", "sort"}

func initGlobals() {

	exe, err0 := os.Executable()
	if err0 != nil {
		panic(err0)
	}

	exPath := filepath.Dir(exe)

	configViper.SetConfigName("service") // name of config file (without extension)
	configViper.SetConfigType("json")    // REQUIRED if the config file does not have the extension in the name
	configViper.AddConfigPath(".")       // optionally look for config in the working directory
	configViper.AddConfigPath(exPath)
	if err := configViper.ReadInConfig(); err != nil {
		logger.Fatal("Could Not Read service.json config file")
	}

	airportsViper.SetConfigName("airports")
	airportsViper.SetConfigType("json")
	airportsViper.AddConfigPath(".") // optionally look for config in the working directory
	airportsViper.AddConfigPath(exPath)
	if err := airportsViper.ReadInConfig(); err != nil {
		logger.Fatal("Could Not Read airports.json config file")
	}

	userViper.SetConfigName("users")
	userViper.SetConfigType("json")
	userViper.AddConfigPath(".") // optionally look for config in the working directory
	userViper.AddConfigPath(exPath)
	if err := userViper.ReadInConfig(); err != nil {
		logger.Fatal("Could Not Read users.json config file")
	}
	userViper.OnConfigChange(func(e fsnotify.Event) {
		logger.Warn("User Config File Changed. Re-reading it")
		if err := userViper.ReadInConfig(); err != nil {
			logger.Fatal("Could Not Read users.json config file")
		}
	})
	userViper.WatchConfig()

	loc, _ = time.LoadLocation("Local")
	//serviceConfig = getServiceConfig()
	isDebug = configViper.GetBool("DebugService")

	initLogging()

	if configViper.GetBool("EnableMetrics") {
		metricsLogger.SetLevel(logrus.InfoLevel)
	} else {
		metricsLogger.SetLevel(logrus.ErrorLevel)
	}

	logger.SetLevel(logrus.InfoLevel)
	requestLogger.SetLevel(logrus.InfoLevel)

	if isDebug {
		logger.SetLevel(logrus.DebugLevel)
	}

}

func initLogging() {
	logger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if configViper.GetString("LogFile") != "" {
		logger.SetOutput(&lumberjack.Logger{
			Filename:   configViper.GetString("LogFile"),
			MaxSize:    configViper.GetInt("MaxLogFileSizeInM"), // megabytes
			MaxBackups: configViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
	requestLogger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if configViper.GetString("RequestLogFile") != "" {
		requestLogger.SetOutput(&lumberjack.Logger{
			Filename:   configViper.GetString("RequestLogFile"),
			MaxSize:    configViper.GetInt("MaxLogFileSizeInMB"), // megabytes
			MaxBackups: configViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
	metricsLogger.Formatter = &easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05.000000",
		LogFormat:       "[%lvl%]: %time% - %msg%\n",
	}
	if configViper.GetString("MetricsLogFile") != "" {
		metricsLogger.SetOutput(&lumberjack.Logger{
			Filename:   configViper.GetString("MetricsLogFile"),
			MaxSize:    configViper.GetInt("MaxLogFileSizeInMB"), // megabytes
			MaxBackups: configViper.GetInt("MaxNumberLogFiles"),
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		})
	}
}
