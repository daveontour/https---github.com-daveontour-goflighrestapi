package main

import (
	"flightresourcerestapi/cmd"
	"flightresourcerestapi/globals"
	"flightresourcerestapi/timeservice"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
)

func main() {

	// Do a bit of initialisation
	globals.InitGlobals()
	timeservice.InitTimeService()

	inService, err := svc.IsWindowsService()

	if err != nil {
		log.Fatalf("Failed to determine if we are running in service: %v", err)
	}

	if inService {
		cmd.RunService(globals.ConfigViper.GetString("ServiceName"), false)
		return
	}

	//Sets up the CLI
	cmd.InitCobra()

	//Invoke the CLI
	cmd.ExecuteCobra()
}
