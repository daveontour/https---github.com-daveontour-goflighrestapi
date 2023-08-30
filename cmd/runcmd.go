package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/version"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

func InitCobra() {

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.Version = version.Version

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(splashCmd)
	rootCmd.AddCommand(demoCmd)
	rootCmd.AddCommand(perfTestCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
}
func ExecuteCobra() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "flightresourcerestapi",
	Short: `flightresourcerestapi is a CLI to run and manage the flights and resource API`,
	Long:  "\nflightresourcerestapi is a CLI to control the execution of the Flight and Resource Rest Service API for AMS\nThe service sits in front of SITA AMS (versions 6.6.x and 6.7.x)\nThe APIs are accessed via HTTP REST API calls",
}
var runCmd = &cobra.Command{
	Use:   "run",
	Short: `Runs the service from the command line`,
	Long:  `Runs the service from the command line. Administrator access is NOT required (unless using port 80) `,
	Run: func(cmds *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(0)
		RunService(globals.ConfigViper.GetString("ServiceName"), true)
	},
}
var demoCmd = &cobra.Command{
	Use:   "demo  {number of flights to create} {number of custom properties}",
	Short: `Run in Demonstration mode`,
	Long:  "\nThis will run the system in demonstration mode where resources and flights will be created based on the configuration in test.json\nThis does not require RabbitMQ or AMS to execute, but the full functionality of the API is available",
	Run: func(cmds *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(2)
		demo()
	},
}
var perfTestCmd = &cobra.Command{
	Use:   "perfTest",
	Short: `Run in Performance Testing mode`,
	Long:  "\nThis will run the system in demonstration mode where resources and flights will be created based on the configuration in test.json\nRabbit MQ is required, but AMS is not required",
	Run: func(cmds *cobra.Command, args []string) {
		globals.IsDebug = true
		splash(0)
		perfTest()
	},
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
func RunService(name string, isDebug bool) {
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

func eventMonitor() {

	//Acts as an exchange between events and action to be taken on those events

	for {
		select {
		case flightChanMesage := <-globals.FlightUpdatedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightUpdated: %s", flightChanMesage.FlightID))
			go repo.HandleFlightUpdate(flightChanMesage)

		case flight := <-globals.FlightDeletedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightDeleted: %s", flight.GetFlightID()))
			go repo.HandleFlightDelete(flight)

		case flightChanMesage := <-globals.FlightUpdatedChannel:

			globals.Logger.Trace(fmt.Sprintf("FlightCreated: %s", flightChanMesage.FlightID))
			go repo.HandleFlightCreate(flightChanMesage)

		case numflight := <-globals.FlightsInitChannel:
			globals.Logger.Trace(fmt.Sprintf("Flight Initialised or Refreshed: %v", numflight))

		case fileDelete := <-globals.FileDeleteChannel:
			go deleteFile(fileDelete)
		}
	}
}

func deleteFile(fileDelete string) {

	fmt.Println("Starting File Deleted ", fileDelete)
	deleted := false
	_, existErr := os.Stat(fileDelete)
	if existErr != nil {
		deleted = true
	}

	for !deleted {
		time.Sleep(5 * time.Second)
		err := os.Remove(fileDelete)
		if err == nil {
			deleted = true
		}
		_, existErr := os.Stat(fileDelete)
		if existErr != nil {
			deleted = true
		}
	}
	fmt.Println("File Deleted ", fileDelete)
}
