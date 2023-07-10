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

	"log"

	"golang.org/x/sys/windows/svc"
)

var repoMap = make(map[string]Repository)
var wg sync.WaitGroup

var repoMutex sync.Mutex
var serviceConfig ServiceConfig

var repositoryUpdateChannel = make(chan int)
var flightUpdatedChannel = make(chan Flight)
var flightCreatedChannel = make(chan Flight)
var flightDeletedChannel = make(chan Flight)

var flightList = FlightList{}

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

func main() {

	serviceConfig = getServiceConfig()
	svcName := serviceConfig.ServiceName

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
		//runProgram()
	}

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
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

// func testDupAndPrune(flight Flight) {

// 	flDup := flight.DuplicateFlight()

// 	properties := make(map[string]string)

// 	for _, p := range flDup.FlightState.Value {
// 		properties[p.PropertyName] = p.Text
// 	}

// 	flDup.FlightState.Value = []Value{}

// 	allowedCustomFields := []string{"FlightUniqueID", "SYS_ETA", "de--_ActualArrival_Source00"}

// 	for _, property := range allowedCustomFields {
// 		data, ok := properties[property]

// 		if ok {
// 			flDup.FlightState.Value = append(flDup.FlightState.Value, Value{property, data})
// 		}
// 	}

// 	changes := []Change{}

// 	for ii := 0; ii < len(flDup.FlightChanges.Changes); ii++ {
// 		ok := contains(allowedCustomFields, flDup.FlightChanges.Changes[ii].PropertyName)
// 		if ok {
// 			changes = append(changes, flDup.FlightChanges.Changes[ii])
// 		}
// 	}

// 	flDup.FlightChanges.Changes = changes

// 	b, err := json.MarshalIndent(flDup, "", "  ")
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		fmt.Println(string(b))
// 	}
// }

// func contains(elems []string, v string) bool {
// 	for _, s := range elems {
// 		if v == s {
// 			return true
// 		}
// 	}
// 	return false
// }
