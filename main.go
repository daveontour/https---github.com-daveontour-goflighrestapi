package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"flag"

	//"log"
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
)

// var repoMap = make(map[string]Repository)
var repoList []Repository
var wg sync.WaitGroup

var repoMutex sync.Mutex
var mapMutex sync.Mutex
var serviceConfig ServiceConfig
var isDebug bool = false

var logger = logrus.New()

var repositoryUpdateChannel = make(chan int)
var flightUpdatedChannel = make(chan Flight)
var flightCreatedChannel = make(chan Flight)
var flightDeletedChannel = make(chan Flight)
var flightsInitChannel = make(chan int)

var schedulerMap = make(map[string]*gocron.Scheduler)

//var flightList = FlightList{}

func GetRepo(airportCode string) *Repository {
	for _, repo := range repoList {
		if repo.Airport == airportCode {
			return &repo
		}
	}
	return nil
}
func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop, pause or continue.\n",
		errmsg, os.Args[0])
	os.Exit(2)
}

func getServiceConfig() ServiceConfig {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, "service.json"))
	byteResult, _ := ioutil.ReadAll(fileContent)

	var serviceConfig ServiceConfig
	json.Unmarshal([]byte(byteResult), &serviceConfig)

	return serviceConfig
}

// func main() {

// 	startGinServer()

// }

func main() {

	logger.SetLevel(log.InfoLevel)
	logger.Info("STARTING PROGRAM")
	serviceConfig = getServiceConfig()
	svcName := serviceConfig.ServiceName
	isDebug = serviceConfig.DebugService

	flag.StringVar(&svcName, "name", svcName, "name of the service")
	flag.Parse()

	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}
	if inService {
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		isDebug = true
		runService(svcName, true)
		return
	case "install":
		err = installService(svcName, serviceConfig.ServicDisplayName, serviceConfig.ServiceDescription)
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(svcName, svc.Pause, svc.Paused)
	case "continue":
		err = controlService(svcName, svc.Continue, svc.Running)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
	return
}
