package cmd

import (
	"flightresourcerestapi/version"
	"fmt"

	"github.com/spf13/cobra"
)

var splashCmd = &cobra.Command{
	Use:   "splash",
	Short: `Shows the Splash text`,
	Long:  `Shows the Splash text`,
	Run: func(cmd *cobra.Command, args []string) {
		splash(0)
	},
}

func splash(mode int) {
	fmt.Println()
	fmt.Println("*******************************************************")
	fmt.Println("*                                                     *")
	fmt.Println("*  AMS Flights and Resources Rest API (" + version.Version + ")         * ")
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
