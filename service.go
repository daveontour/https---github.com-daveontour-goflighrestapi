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
	"golang.org/x/sys/windows/svc/eventlog"
)

var elog debug.Log
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
				elog.Info(1, testOutput)

				//Stop the Servers
				wg.Done()
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}

			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

func runProgram() {

	numCPU := runtime.NumCPU()

	if isDebug {
		elog.Info(1, fmt.Sprintf("Number of cores available = %v", numCPU))
	}
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
			if isDebug {
				elog.Info(59, fmt.Sprintf("FlightUpdated: %s", flight.GetFlightID()))
			}
		case flight := <-flightDeletedChannel:
			if isDebug {
				elog.Info(59, fmt.Sprintf("FlightDeleted: %s", flight.GetFlightID()))
			}
		case flight := <-flightCreatedChannel:
			if isDebug {
				elog.Info(59, fmt.Sprintf("FlightCreated: %s", flight.GetFlightID()))
			}
		case numflight := <-flightsInitChannel:
			if isDebug {
				elog.Info(59, fmt.Sprintf("Flight Initialised or Refreshed: %v", numflight))
			}
		}
	}
}
