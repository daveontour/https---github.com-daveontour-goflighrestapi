package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
)

var repoMap = make(map[string]Repository)
var wg sync.WaitGroup

var repoMutex sync.Mutex

var repositoryUpdateChannel = make(chan int)
var flightUpdatedChannel = make(chan Flight)
var flightCreatedChannel = make(chan Flight)
var flightDeletedChannel = make(chan Flight)

func main() {

	numCPU := runtime.NumCPU()

	wg.Add(1)

	fmt.Println("Number of cores available = ", numCPU)
	runtime.GOMAXPROCS(runtime.NumCPU())

	go startGinServer()
	go cup()
	go InitRepositories()

	wg.Wait()

}

func cup() {

	for {
		select {
		case c := <-repositoryUpdateChannel:
			fmt.Print("Repository Channel Update", c)
		case flight := <-flightUpdatedChannel:
			fmt.Println("FlightUpdated:", flight.GetFlightID())
		case flight := <-flightDeletedChannel:
			fmt.Println("FlightDeleted:", flight.GetFlightID())
		case flight := <-flightCreatedChannel:
			fmt.Println("FlightCreated:", flight.GetFlightID())
		}
	}
}

func testDupAndPrune(flight Flight) {

	flDup := flight.DuplicateFlight()

	properties := make(map[string]string)

	for _, p := range flDup.FlightState.Value {
		properties[p.PropertyName] = p.Text
	}

	flDup.FlightState.Value = []Value{}

	allowedCustomFields := []string{"FlightUniqueID", "SYS_ETA", "de--_ActualArrival_Source00"}

	for _, property := range allowedCustomFields {
		data, ok := properties[property]

		if ok {
			flDup.FlightState.Value = append(flDup.FlightState.Value, Value{property, data})
		}
	}

	changes := []Change{}

	for ii := 0; ii < len(flDup.FlightChanges.Changes); ii++ {
		ok := contains(allowedCustomFields, flDup.FlightChanges.Changes[ii].PropertyName)
		if ok {
			changes = append(changes, flDup.FlightChanges.Changes[ii])
		}
	}

	flDup.FlightChanges.Changes = changes

	b, err := json.MarshalIndent(flDup, "", "  ")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(string(b))
	}
}

func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
