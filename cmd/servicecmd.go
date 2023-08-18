package cmd

import (
	"flightresourcerestapi/globals"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

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
func amAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}
func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
