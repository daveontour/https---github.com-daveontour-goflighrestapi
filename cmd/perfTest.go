package cmd

import (
	"fmt"

	"runtime"
	"strconv"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/server"
)

func perfTest(numFlightsSt string, minCustomPropertiesSt string) {

	// Start the system in performance test mode. Resources and flights are created as per test.json
	// Requires Rabbit MQ to be running. Messages are passsed via Rabbit MQ
	numCPU := runtime.NumCPU()

	globals.Logger.Debug(fmt.Sprintf("Number of cores available = %v", numCPU))

	runtime.GOMAXPROCS(runtime.NumCPU())
	//Wait group so the program doesn't exit
	globals.Wg.Add(1)

	// The HTTP Server
	go server.StartGinServer()

	// Handler for the different types of messages passed by channels
	go eventMonitor()

	// Initiate the User Change Subscriptions
	globals.UserChangeSubscriptionsMutex.Lock()
	for _, up := range globals.GetUserProfiles() {
		if up.Enabled {
			globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
		}
	}
	globals.UserChangeSubscriptionsMutex.Unlock()

	numFlights, _ := strconv.Atoi(numFlightsSt)
	minProps, _ := strconv.Atoi(minCustomPropertiesSt)
	repo.StartChangePushWorkerPool(globals.ConfigViper.GetInt("NumberOfChangePushWorkers"))
	repo.PerfTestInit(numFlights, minProps)
	globals.Wg.Wait()
}
