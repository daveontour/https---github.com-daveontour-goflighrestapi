package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func startGinServer() {

	router := gin.Default()

	router.GET("/getFlights/:apt", getRequestedFlights)
	router.GET("/getFlights/", getRequestedFlights)
	router.Run()

}

func getUserProfile(c *gin.Context) UserProfile {

	//Read read the file for each request so changes can be made without the need to restart the server

	fileContent, err := os.Open("users.json")

	if err != nil {
		log.Fatal(err)
		return UserProfile{}
	}

	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var users Users

	json.Unmarshal([]byte(byteResult), &users)

	keys := c.Request.Header["token"]
	key := "default"

	if keys != nil {
		key = keys[0]
	}

	userProfile := UserProfile{}

	for _, u := range users.Users {
		if key == u.Key {
			userProfile = u
			break
		}
	}

	return userProfile
}

func getRequestedFlights(c *gin.Context) {

	apt := c.Param("apt")

	direction := strings.ToUpper(c.Query("type"))
	airline := c.Query("al")
	from := c.Query("from")
	to := c.Query("to")

	// Create the response object so we can return early if required
	response := Response{}
	c.Writer.Header().Set("Content-Type", "application/json")

	var err error

	// Add the flights the response object and return nil for errors
	if direction != "" {
		response.Direction = direction
	} else {
		response.Direction = "ARR/DEP"
	}

	// Get the profile of the user making the request
	userProfile := getUserProfile(c)
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" && userProfile.DefaultAirport != "" {
		apt = userProfile.DefaultAirport
	}
	// Set override airport if specified in configuration
	if userProfile.OverrideAirport != "" {
		apt = userProfile.OverrideAirport
		response.AddWarning(fmt.Sprintf("Airport set to %s by the administration configuration", apt))
	}

	// Set Default airline if none set
	if airline == "" && userProfile.DefaultAirline != "" {
		airline = userProfile.DefaultAirline
		response.AddWarning(fmt.Sprintf("Airline set to %s by the administration configuration", airline))
	}
	// Set override airline if specified in configuration
	if userProfile.OverrideAirline != "" {
		airline = userProfile.OverrideAirline
		response.AddWarning(fmt.Sprintf("Airline set to %s by the administration configuration", airline))
	}

	//Check that the user is allowed to access the requested airport
	if !contains(userProfile.AllowedAirlines, apt) &&
		!contains(userProfile.AllowedAirlines, "*") {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "User is not permitted to access airport %s"})
		return
	}

	// Check that the requested airport exists inthe repository
	_, ok := repoMap[apt]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Airport %s not found", apt)})
		return
	}

	response.AirportCode = apt

	// Build the request object
	request := Request{Direction: direction, Airline: airline, From: from, To: to, UserProfile: userProfile}

	// Reform the request based on the user Profile and the request parameters
	request, response = normaliseRequest(request, response, c)

	// If the user is requesting a particular airline, check that they are allowed to access that airline
	if airline != "" && userProfile.AllowedAirlines != nil {
		if !contains(userProfile.AllowedAirlines, airline) &&
			!contains(userProfile.AllowedAirlines, "*") {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Request for airline %s not alowed by user", airline)})
			return
		}
	}

	// Get the filtered and pruned flights for the request
	response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	if err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", err.Error())})
	}

}

type MyError struct{}

func (m *MyError) Error() string {
	return "Query against an unauthorised custom field. Refer to administrator"
}

func normaliseRequest(request Request, response Response, c *gin.Context) (Request, Response) {
	if request.UserProfile.AllowedAirlines != nil &&
		!contains(request.UserProfile.AllowedAirlines, "*") {
		response.AddWarning(fmt.Sprintf("Users request restricted by administrator to airlines %s", request.UserProfile.AllowedAirlines))
	}

	presentQueryableParameters := []ParameterValuePair{}
	// Set up the custom parameter queries if required
	if request.UserProfile.QueryableCustomFields != nil {

		queryMap := c.Request.URL.Query()

		// Go through the querable parameters
		for _, queryableParameter := range request.UserProfile.QueryableCustomFields {

			value, present := queryMap[queryableParameter]

			if present && !contains(request.UserProfile.AllowedCustomFields, queryableParameter) {
				response.AddWarning(fmt.Sprintf("Non queryable parameter specified: %s", queryableParameter))
			}

			if present {

				// Check for override

				replace := false
				for _, pair := range request.UserProfile.OverrideQueryableCustomFields {
					if pair.Parameter == queryableParameter {
						presentQueryableParameters = append(presentQueryableParameters, ParameterValuePair{Parameter: queryableParameter, Value: pair.Value})
						replace = true
						response.AddWarning(fmt.Sprintf("Query value of %s replaced with %s by admnistration configuration", pair.Parameter, pair.Value))
						break
					}
				}

				if !replace {
					presentQueryableParameters = append(presentQueryableParameters, ParameterValuePair{Parameter: queryableParameter, Value: value[0]})
				}
			}
		}
		request.PresentQueryableParameters = presentQueryableParameters
	}

	if presentQueryableParameters == nil || len(presentQueryableParameters) == 0 {
		if request.UserProfile.DefaultQueryableCustomFields != nil {
			request.PresentQueryableParameters = request.UserProfile.DefaultQueryableCustomFields
			response.AddWarning("Custom Field query set to default by Administrator Configuration")
		}
	}

	// if request.PresentQueryableParameters != nil &&
	// 	request.UserProfile.OverrideQueryableCustomFields != nil {
	// 	request.PresentQueryableParameters = request.UserProfile.OverrideQueryableCustomFields
	// 	response.AddWarning("Custom Field query overridden by Administrator Configuration")
	// }

	return request, response
}
func filterFlights(request Request, response Response, flights map[string]Flight, c *gin.Context) (Response, error) {

	returnFlights := []Flight{}

	var from time.Time
	var to time.Time

	// Set up the time bounds if required. Return error to user if not well formed date
	if request.From != "" {
		f, err := time.Parse("2006-01-02T15:04:05", request.From)
		if err != nil {
			return Response{}, err
		} else {
			from = f
			response.From = f.String()
		}
	}
	if (request.To) != "" {
		t, err := time.Parse("2006-01-02T15:04:05", request.To)
		if err != nil {
			return Response{}, err
		} else {
			to = t
			response.To = t.String()
		}
	}

	for _, f := range flights {

		passQueryCheck := true
		for _, queryableParameter := range request.PresentQueryableParameters {
			queryValue := queryableParameter.Value
			flightValue := f.GetProperty(queryableParameter.Parameter)
			if flightValue == "" {
				passQueryCheck = false
				continue
			}
			if queryValue != flightValue {
				passQueryCheck = false
				continue
			} else {
				passQueryCheck = true
			}
		}

		if !passQueryCheck {
			continue
		}

		// Flight direction filter
		if (request.Direction == "DEP") && f.IsArrival() {
			continue
		}
		if (request.Direction == "ARR") && !f.IsArrival() {
			continue
		}

		// Requested Airline Code filter
		if request.Airline != "" && f.GetIATAAirline() != request.Airline {
			continue
		}

		// STO window filters
		if request.From != "" {
			if f.GetSTO().Before(from) {
				continue
			}
		}
		if request.To != "" {
			if f.GetSTO().After(to) {
				continue
			}
		}

		// Filter out airlines that the user is not allowed to see
		// "*" entry in AllowedAirlines allows all.
		if request.UserProfile.AllowedAirlines != nil {
			if !contains(request.UserProfile.AllowedAirlines, f.GetIATAAirline()) &&
				!contains(request.UserProfile.AllowedAirlines, "*") {
				continue
			}
		}

		// Made it here without being filtered out, so add it to the flights to be returned. The "prune"
		// function removed any message elements that the user is not allowed to see
		returnFlights = append(returnFlights, prune(f, request))
	}

	response.Flights = returnFlights
	response.NumberOfFlights = len(returnFlights)
	response.CustomFieldQuery = request.PresentQueryableParameters

	return response, nil
}

func prune(flight Flight, request Request) Flight {

	/*
	 *   Creates a copy of the flight record with the custom fields that the user is allowed to see
	 */

	flDup := flight.DuplicateFlight()

	flDup.FlightState.Value = []Value{}

	// If Allowed CustomFields is not nil, then filter the custome fields
	// if "*" in list then it is all custom fields
	// Extra safety, if the parameter is not defined, then no results returned

	if request.UserProfile.AllowedCustomFields != nil {
		if contains(request.UserProfile.AllowedCustomFields, "*") {
			// No restriction is defined on the custom fields, so let it rip
			for _, property := range flight.FlightState.Value {
				data := flight.GetProperty(property.PropertyName)

				if data != "" {
					flDup.FlightState.Value = append(flDup.FlightState.Value, Value{property.PropertyName, data})
				}
			}
		} else {
			for _, property := range request.UserProfile.AllowedCustomFields {
				data := flight.GetProperty(property)

				if data != "" {
					flDup.FlightState.Value = append(flDup.FlightState.Value, Value{property, data})
				}
			}
		}
	}

	changes := []Change{}

	for ii := 0; ii < len(flDup.FlightChanges.Changes); ii++ {
		ok := contains(request.UserProfile.AllowedCustomFields, flDup.FlightChanges.Changes[ii].PropertyName)
		ok = ok || request.UserProfile.AllowedCustomFields == nil
		if ok {
			changes = append(changes, flDup.FlightChanges.Changes[ii])
		}
	}

	flDup.FlightChanges.Changes = changes

	return flDup
}
