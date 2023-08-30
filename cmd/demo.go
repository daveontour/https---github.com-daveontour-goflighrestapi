package cmd

import (
	"runtime"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/server"
)

func demo() {

	// Start the system in demo mode. Resources and flights are created as per test.json
	// Does not require Rabbit MQ to be running.
	globals.DemoMode = true

	runtime.GOMAXPROCS(runtime.NumCPU())
	globals.Wg.Add(1)
	go server.StartGinServer(true)
	go eventMonitor()

	// // Initiate the User Change Subscriptions
	globals.UserChangeSubscriptionsMutex.Lock()
	for _, up := range globals.GetUserProfiles() {
		if up.Enabled {
			globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
		}
	}
	globals.UserChangeSubscriptionsMutex.Unlock()

	repo.StartChangePushWorkerPool(globals.ConfigViper.GetInt("NumberOfChangePushWorkers"))
	repo.PerfTestInit()

	repo.SchedulePushes("APT", true)
	globals.Wg.Wait()
}
