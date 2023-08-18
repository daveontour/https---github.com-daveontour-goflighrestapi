package main

import (
	"errors"
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
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const version = "2.1.0"

var rootCmd = &cobra.Command{
	Use:   "flightresourcerestapi",
	Short: `flightresourcerestapi is a CLI to run or manage the flights and resource API`,
	Long:  `flightresourcerestapi is a CLI to control the execution of the Flight and Resource Rest Service API for AMS`,
	// Run: func(cmd *cobra.Command, args []string) {
	// 	globals.IsDebug = true
	// 	splash(0)
	// },
}

var splashCmd = &cobra.Command{
	Use:   "splash",
	Short: `Shows the Splash text`,
	Long:  `Shows the Splash text`,
	Run: func(cmd *cobra.Command, args []string) {
		splash(0)
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: `Runs the service from the command line`,
	Long:  `Runs the service from the command line. Administrator access is NOT required (unless using port 80) `,
	Run: func(cmd *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(0)
		runService(globals.ConfigViper.GetString("ServiceName"), true)
	},
}
var demoCmd = &cobra.Command{
	Use:   "demo  {number of flights to create}",
	Short: `Run in Demonstration mode`,
	Long:  `This will run the system in demonstration mode where resources and flights will be created based on the configuration in test.json`,
	Run: func(cmd *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(2)
		demo(args[0])
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Number of initial flights not specified")
		}
		_, err := strconv.Atoi(args[0])
		if err != nil {
			return errors.New("Invalid format or invalid number of flights entered on command line")
		}
		return nil
	},
}
var perfTestCmd = &cobra.Command{
	Use:   "perfTest {number of flights to create}",
	Short: `Run in Performance Testing mode`,
	Long:  `This will run the system in demonstration mode where resources and flights will be created based on the configuration in test.json`,
	Run: func(cmd *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(0)
		perfTest(args[1])
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Number of initial flights not specified")
		}
		_, err := strconv.Atoi(args[0])
		if err != nil {
			return errors.New("Invalid format or invalid number of flights entered on command line")
		}
		return nil
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: `Install to run as a Windows Service (Adminstrator Mode Required)`,
	Long:  `Install the system to run as a Windows Service. Must be logged on as Administrator`,
	Run: func(cmd *cobra.Command, args []string) {
		if !amAdmin() {
			fmt.Println("Administrator privilge required")
			return
		}
		err := installService(globals.ConfigViper.GetString("ServiceName"), globals.ConfigViper.GetString("ServicDisplayName"), globals.ConfigViper.GetString("ServiceDescription"))
		failOnError(err, fmt.Sprintf("failed to %s %s", "install", globals.ConfigViper.GetString("ServiceName")))
	},
}
var removeCmd = &cobra.Command{
	Use:   "uninstall",
	Short: `Uninstalls the system if previously installed as a Windows Service (Adminstrator Mode Required)`,
	Long:  `Uninstalls the system if previously installed as a Windows Service. Must be logged on as Administrator`,
	Run: func(cmd *cobra.Command, args []string) {
		if !amAdmin() {
			fmt.Println("Administrator privilge required")
			return
		}
		err := removeService(globals.ConfigViper.GetString("ServiceName"))
		failOnError(err, fmt.Sprintf("failed to %s %s", "uninstall", globals.ConfigViper.GetString("ServiceName")))
	},
}
var startCmd = &cobra.Command{
	Use:   "start",
	Short: `Starts the service if previously installed as a Windows Service (Adminstrator Mode Required)`,
	Long:  `Starts the service if previously installed as a Windows Service. Must be logged on as Administrator`,
	Run: func(cmd *cobra.Command, args []string) {
		if !amAdmin() {
			fmt.Println("Administrator privilge required")
			return
		}
		err := startService(globals.ConfigViper.GetString("ServiceName"))
		failOnError(err, fmt.Sprintf("failed to %s %s", "start", globals.ConfigViper.GetString("ServiceName")))
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: `Stops the service if previously installed as a Windows Service (Adminstrator Mode Required)`,
	Long:  `Stops the service if previously installed as a Windows Service. Must be logged on as Administrator`,
	Run: func(cmd *cobra.Command, args []string) {
		if !amAdmin() {
			fmt.Println("Administrator privilge required")
			return
		}
		err := controlService(globals.ConfigViper.GetString("ServiceName"), svc.Stop, svc.Stopped)
		failOnError(err, fmt.Sprintf("failed to %s %s", "stop", globals.ConfigViper.GetString("ServiceName")))
	},
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func amAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.Version = version

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(splashCmd)
	rootCmd.AddCommand(demoCmd)
	rootCmd.AddCommand(perfTestCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
func main() {

	// Do a bit of initialisation
	globals.InitGlobals()
	timeservice.InitTimeService()

	inService, err := svc.IsWindowsService()

	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}

	if inService {
		// Running as a Windows service, so just go ahead and start it all
		runService(globals.ConfigViper.GetString("ServiceName"), false)
		return
	}

	Execute()

}

func splash(mode int) {
	fmt.Println()
	fmt.Println("*******************************************************")
	fmt.Println("*                                                     *")
	fmt.Println("*  AMS Flights and Resources Rest API (" + version + ")         * ")
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
		globals.UserChangeSubscriptions = append(globals.UserChangeSubscriptions, up.UserChangeSubscriptions...)
	}
	globals.UserChangeSubscriptionsMutex.Unlock()

	numFlights, _ := strconv.Atoi(numFlightsSt)
	repo.PerfTestInit(numFlights)
	globals.Wg.Wait()
}

func demo(numFlightsSt string) {

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
	repo.PerfTestInit(numFlights)
	globals.Wg.Wait()
}

func eventMonitor() {

	//Acts as an exchange between events and action to be taken on those events

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
