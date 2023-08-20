package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/server"
)

func demo(numFlightsSt string, minCustomPropertiesSt string) {

	// Start the system in demo mode. Resources and flights are created as per test.json
	// Does not require Rabbit MQ to be running.
	globals.DemoMode = true

	runtime.GOMAXPROCS(runtime.NumCPU())
	globals.Wg.Add(1)
	go server.StartGinServer()
	go eventMonitor()

	// // Initiate the User Change Subscriptions
	globals.UserChangeSubscriptionsMutex.Lock()
	for _, up := range globals.GetUserProfiles() {
		globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
	}
	globals.UserChangeSubscriptionsMutex.Unlock()

	numFlights, err := strconv.Atoi(numFlightsSt)
	if err != nil {
		fmt.Println("Invalid number of flights entered on command line")
		os.Exit(0)
	}
	minProps, _ := strconv.Atoi(minCustomPropertiesSt)
	repo.StartChangePushWorkerPool(globals.ConfigViper.GetInt("NumberOfChangePushWorkers"))
	repo.PerfTestInit(numFlights, minProps)

	repo.SchedulePushes("APT", true)
	globals.Wg.Wait()
}
