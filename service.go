// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	// "github.com/sirupsen/logrus"
	//"golang.org/x/sys/windows/svc/eventlog"
)

// var logger debug.Log
var serviceRunning = false

type exampleService struct{}

func (m *exampleService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	//fasttick := time.Tick(500 * time.Millisecond)
	//tick := fasttick
	//started := false

	go runProgram()
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {

		select {

		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// golang.org/x/sys/windows/svc.TestExample is verifying this output.
				testOutput := strings.Join(args, "-")
				testOutput += fmt.Sprintf("-%d", c.Context)
				logger.Debug(testOutput)

				//Stop the Servers
				wg.Done()
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}

			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
				logger.Error(fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error
	// if isDebug {
	// 	logger = debug.New(name)
	// } else {
	// 	logger, err = eventlog.Open(name)
	// 	if err != nil {
	// 		return
	// 	}
	// }
	// defer logger.Close()

	logger.Info(fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		logger.Info(fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	logger.Info(fmt.Sprintf("%s service stopped", name))
}

func runProgram() {

	numCPU := runtime.NumCPU()

	logger.Debug(fmt.Sprintf("Number of cores available = %v", numCPU))

	runtime.GOMAXPROCS(runtime.NumCPU())
	wg.Add(1)
	go startGinServer()
	go eventMonitor()
	go InitRepositories()
	wg.Wait()
}

func eventMonitor() {

	for {
		select {
		case flight := <-flightUpdatedChannel:

			logger.Trace(fmt.Sprintf("FlightUpdated: %s", flight.GetFlightID()))
			go handleFlightUpdate(flight)

		case flight := <-flightDeletedChannel:

			logger.Trace(fmt.Sprintf("FlightDeleted: %s", flight.GetFlightID()))
			go handleFlightDelete(flight)

		case flight := <-flightCreatedChannel:

			logger.Trace(fmt.Sprintf("FlightCreated: %s", flight.GetFlightID()))
			go handleFlightCreate(flight)

		case numflight := <-flightsInitChannel:

			logger.Trace(fmt.Sprintf("Flight Initialised or Refreshed: %v", numflight))

		}
	}
}
