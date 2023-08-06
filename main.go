package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

func main() {

	initGlobals()

	svcName := configViper.GetString("ServiceName")
	inService, err := svc.IsWindowsService()

	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}
	if inService {
		runService(svcName, false)
		return
	}

	cmd := ""
	if len(os.Args) >= 2 {
		cmd = strings.ToLower(os.Args[1])
	}

	switch cmd {
	case "debug":
		isDebug = true
		splash()
		runService(svcName, true)
		return
	case "install":
		err = installService(svcName, configViper.GetString("ServicDisplayName"), configViper.GetString("ServiceDescription"))
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	default:
		splash()
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

func splash() {
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
	fmt.Println("*******************************************************")
	fmt.Println()
}

func installService(name, displayName, desc string) error {
	exepath, err := exePath()
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
	s, err := m.OpenService(configViper.GetString("ServiceName"))
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
				logger.Debug(testOutput)

				//Stop the Servers
				wg.Done()
				break loop
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

	logger.Info(fmt.Sprintf("Starting %s service", name))
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
	//Wait group so the program doesn't exit
	wg.Add(1)

	// The HTTP Server
	go startGinServer()

	// Handler for the different types of messages passed by channels
	go eventMonitor()

	// Manages the population and update of the repositoiry of flights
	go InitRepositories()

	// Initiate the User Change Subscriptions
	userChangeSubscriptionsMutex.Lock()
	for _, up := range getUserProfiles() {
		userChangeSubscriptions = append(userChangeSubscriptions, up.UserChangeSubscriptions...)
	}
	userChangeSubscriptionsMutex.Unlock()
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
