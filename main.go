package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/server"
	"flightresourcerestapi/timeservice"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

func main() {

	// Do a bit of initialisation
	globals.InitGlobals()
	timeservice.InitTimeService()

	svcName := globals.ConfigViper.GetString("ServiceName")
	inService, err := svc.IsWindowsService()

	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}

	if inService {
		// Running as a Windows service, so just go ahead and start it all
		runService(svcName, false)
		return
	}

	// Not in a service, so process the command line
	cmd := ""
	if len(os.Args) >= 2 {
		cmd = strings.ToLower(os.Args[1])
	}

	switch cmd {
	case "debug":
		globals.IsDebug = true
		splash(0)
		runService(svcName, true)
		return
	case "perftest":
		globals.IsDebug = true
		splash(1)
		if len(os.Args) < 3 {
			fmt.Println("You must specify the number of flights to include in the test")
			return
		}
		perfTest(os.Args[2])
		return
	case "demo":
		globals.IsDebug = true
		splash(2)
		if len(os.Args) < 3 {
			fmt.Println("You must specify the number of flights to include in the demo")
			return
		}
		demo(os.Args[2])
		return
	case "install":
		err = installService(svcName, globals.ConfigViper.GetString("ServicDisplayName"), globals.ConfigViper.GetString("ServiceDescription"))
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	default:
		globals.IsDebug = true
		splash(0)
		runService(svcName, true)

	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
	return
}

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop\n",
		os.Args[0])
}

func splash(mode int) {
	fmt.Println()
	fmt.Println("*******************************************************")
	fmt.Println("*                                                     *")
	fmt.Println("*  AMS Flights and Resources Rest API  (v2.1.0)       *")
	fmt.Println("*                                                     *")
	fmt.Println("*  (This is NOT official SITA Software)               *")
	fmt.Println("*  (Community Contributed Software)                   *")
	fmt.Println("*                                                     *")
	fmt.Println("*  Responds to HTTP Get Requests for flight and       *")
	fmt.Println("*  resources allocation information                   *")
	fmt.Println("*                                                     *")
	fmt.Println("*  Subscribed users can also receive scheduled push   *")
	fmt.Println("*  notifcations and pushes on changes                 *")
	fmt.Println("*                                                     *")
	fmt.Println("*  See help.html for API usage                        *")
	fmt.Println("*  See adminhelp.html for configuration usage         *")
	fmt.Println("*                                                     *")

	if mode == 1 {
		fmt.Println("*  WARNING! - Running in Performance Test Mode        *")
		fmt.Println("*                                                     *")
	}
	if mode == 2 {
		fmt.Println("*  WARNING! - Running in Demonstration Mode           *")
		fmt.Println("*  Data is fictious and there is no AMS interation    *")
		fmt.Println("*                                                     *")
	}
	fmt.Println("*******************************************************")
	fmt.Println()
}

func installService(name, displayName, desc string) error {
	exepath, err := globals.ExePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}

	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: displayName, Description: desc}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}

	//serviceConfig := getServiceConfig()

	defer m.Disconnect()
	s, err := m.OpenService(globals.ConfigViper.GetString("ServiceName"))
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}
func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

type exampleService struct{}

func (m *exampleService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}

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
				globals.Logger.Debug(testOutput)

				//Stop the Servers
				globals.Wg.Done()
				break loop
			default:
				globals.Logger.Error(fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error

	globals.Logger.Info(fmt.Sprintf("Starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		globals.Logger.Info(fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	globals.Logger.Info(fmt.Sprintf("%s service stopped", name))
}
func runProgram() {

	numCPU := runtime.NumCPU()

	globals.Logger.Debug(fmt.Sprintf("Number of cores available = %v", numCPU))

	runtime.GOMAXPROCS(runtime.NumCPU())
	//Wait group so the program doesn't exit
	globals.Wg.Add(1)

	// The HTTP Server
	go server.StartGinServer()

	// Handler for the different types of messages passed by channels
	go eventMonitor()

	// Manages the population and update of the repositoiry of flights
	go repo.InitRepositories()

	// Initiate the User Change Subscriptions
	globals.UserChangeSubscriptionsMutex.Lock()
	for _, up := range globals.GetUserProfiles() {
		globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
	}
	globals.UserChangeSubscriptionsMutex.Unlock()
	globals.Wg.Wait()
}
func perfTest(numFlightsSt string) {

	numCPU := runtime.NumCPU()

	globals.ConfigViper.Set("PerfTestOnly", true)

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
		globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
	}
	globals.UserChangeSubscriptionsMutex.Unlock()

	numFlights, _ := strconv.Atoi(numFlightsSt)
	repo.PerfTestInit(numFlights)
	globals.Wg.Wait()
}

func demo(numFlightsSt string) {

	globals.DemoMode = true
	globals.ConfigViper.Set("PerfTestOnly", true)
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
	repo.PerfTestInit(numFlights)
	globals.Wg.Wait()
}

func eventMonitor() {

	for {
		select {
		case flight := <-globals.FlightUpdatedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightUpdated: %s", flight.GetFlightID()))
			go repo.HandleFlightUpdate(flight)

		case flight := <-globals.FlightDeletedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightDeleted: %s", flight.GetFlightID()))
			go repo.HandleFlightDelete(flight)

		case flight := <-globals.FlightCreatedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightCreated: %s", flight.GetFlightID()))
			go repo.HandleFlightCreate(flight)

		case numflight := <-globals.FlightsInitChannel:

			globals.Logger.Trace(fmt.Sprintf("Flight Initialised or Refreshed: %v", numflight))

		}
	}
}
