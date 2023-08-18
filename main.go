package main

import (
	"flightresourcerestapi/cmd"
	"flightresourcerestapi/globals"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
)

func main() {

	cmd.InitCobra()

	inService, err := svc.IsWindowsService()

	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}

	if inService {
		cmd.RunService(globals.ConfigViper.GetString("ServiceName"), false)
		return
	}

	cmd.ExecuteCobra()
}
