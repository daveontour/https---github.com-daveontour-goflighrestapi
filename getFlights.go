package main

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func getRequestedFlightsAPI(c *gin.Context) {

	apt := c.Param("apt")
	direction := strings.ToUpper(c.Query("direction"))
	if direction == "" {
		direction = strings.ToUpper(c.Query("d"))
	}
	airline := c.Query("al")
	from := c.Query("from")
	to := c.Query("to")
	route := strings.ToUpper(c.Query("route"))
	if route == "" {
		route = c.Query("r")
	}

	response, error := getRequestedFlightsCommon(apt, direction, airline, from, to, route, "", c, nil)
	c.Writer.Header().Set("Content-Type", "application/json")

	if error.Err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", error.Err.Error())})
	}
}

func getRequestedFlightsSub(sub UserPushSubscription, userToken string) (Response, GetFlightsError) {
	apt := sub.Airport
	direction := strings.ToUpper(sub.Direction)
	airline := sub.Airline
	from := sub.From
	to := sub.To
	route := strings.ToUpper(sub.Route)
	qf := sub.QueryableCustomFields

	return getRequestedFlightsCommon(apt, direction, airline, strconv.Itoa(from), strconv.Itoa(to), route, userToken, nil, qf)

}
func getRequestedFlightsCommon(apt, direction, airline, from, to, route, userToken string, c *gin.Context, qf []ParameterValuePair) (Response, GetFlightsError) {

	// Create the response object so we can return early if required
	response := Response{}

	// Add the flights the response object and return nil for errors
	if direction != "" {
		if strings.HasPrefix(direction, "A") {
			response.Direction = "Arrival"
		}
		if strings.HasPrefix(direction, "D") {
			response.Direction = "Departure"
		}
	} else {
		response.Direction = "ARR/DEP"
	}

	response.Route = route

	// Get the profile of the user making the request
	userProfile := getUserProfile(c, userToken)
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
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("User is not allowed to access requested airline"),
		}
	}

	// Check that the requested airport exists in the repository
	//	_, ok := repoMap[apt]
	if GetRepo(apt) == nil {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New(fmt.Sprintf("Airport %s not found", apt)),
		}
	}

	response.AirportCode = apt

	// Build the request object
	request := Request{Direction: direction, Airline: airline, From: from, To: to, UserProfile: userProfile, Route: route}

	// Reform the request based on the user Profile and the request parameters
	request, response = normaliseRequest(request, response, c, qf)

	// If the user is requesting a particular airline, check that they are allowed to access that airline
	if airline != "" && userProfile.AllowedAirlines != nil {
		if !contains(userProfile.AllowedAirlines, airline) &&
			!contains(userProfile.AllowedAirlines, "*") {
			return response, GetFlightsError{
				StatusCode: http.StatusBadRequest,
				Err:        errors.New("unavailable"),
			}
		}
	}

	var err error
	// Get the filtered and pruned flights for the request
	response, err = filterFlights(request, response, GetRepo(apt).Flights, c)

	if err == nil {
		return response, GetFlightsError{
			StatusCode: http.StatusOK,
			Err:        nil,
		}
	} else {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        err,
		}
	}
}

func normaliseRequest(request Request, response Response, c *gin.Context, qf []ParameterValuePair) (Request, Response) {
	if request.UserProfile.AllowedAirlines != nil &&
		!contains(request.UserProfile.AllowedAirlines, "*") {
		response.AddWarning(fmt.Sprintf("Users request restricted by administrator to airlines %s", request.UserProfile.AllowedAirlines))
	}

	presentQueryableParameters := []ParameterValuePair{}
	// Set up the custom parameter queries if required
	if request.UserProfile.QueryableCustomFields != nil {

		if c != nil {
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
		} else if qf != nil {
			for _, pvPair := range qf {

				parameter := pvPair.Parameter
				value := pvPair.Value

				// Check for override

				replace := false
				for _, pair := range request.UserProfile.OverrideQueryableCustomFields {
					if pair.Parameter == parameter {
						presentQueryableParameters = append(presentQueryableParameters, ParameterValuePair{Parameter: parameter, Value: pair.Value})
						replace = true
						response.AddWarning(fmt.Sprintf("Query value of %s replaced with %s by admnistration configuration", pair.Parameter, pair.Value))
						break
					}
				}

				if !replace {
					presentQueryableParameters = append(presentQueryableParameters, ParameterValuePair{Parameter: parameter, Value: value})
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

	return request, response
}
func filterFlights(request Request, response Response, flights map[string]Flight, c *gin.Context) (Response, error) {

	returnFlights := []Flight{}

	// var from time.Time
	// var to time.Time
	var updatedSinceTime time.Time

	fromOffset, fromErr := strconv.Atoi(request.From)
	if fromErr != nil {
		fromOffset = -12
	}

	from := time.Now().Add(time.Hour * time.Duration(fromOffset))
	response.From = from.Format("2006-01-02T15:04:05")

	toOffset, toErr := strconv.Atoi(request.To)
	if toErr != nil {
		toOffset = 24
	}

	to := time.Now().Add(time.Hour * time.Duration(toOffset))
	response.To = to.Format("2006-01-02T15:04:05")

	if request.UpdatedSince != "" {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", request.UpdatedSince, loc)
		if err != nil {
			return Response{}, err
		} else {
			updatedSinceTime = t
			response.To = t.String()
		}
	}

	allowedAllAirline := false
	if request.UserProfile.AllowedAirlines != nil {
		if contains(request.UserProfile.AllowedAirlines, "*") {
			allowedAllAirline = true
		}
	}

	mapMutex.Lock()

NextFlight:
	for _, f := range flights {

		for _, queryableParameter := range request.PresentQueryableParameters {
			queryValue := queryableParameter.Value
			flightValue := f.GetProperty(queryableParameter.Parameter)
			if flightValue == "" {
				break NextFlight
			}
			if queryValue != flightValue {
				break NextFlight
			}
		}

		// Flight direction filter
		if strings.HasPrefix(request.Direction, "D") && f.IsArrival() {
			continue
		}
		if strings.HasPrefix(request.Direction, "A") && !f.IsArrival() {
			continue
		}

		// Requested Airline Code filter
		if request.Airline != "" && f.GetIATAAirline() != request.Airline {
			continue
		}

		// RequestedRoute filter
		if request.Route != "" && !strings.Contains(f.GetFlightRoute(), request.Route) {
			continue
		}

		if f.GetSTO().Before(from) {
			continue
		}

		if f.GetSTO().After(to) {
			continue
		}

		if request.UpdatedSince != "" {
			if f.LastUpdate.Before(updatedSinceTime) {
				continue
			}
		}

		// Filter out airlines that the user is not allowed to see
		// "*" entry in AllowedAirlines allows all.
		if !allowedAllAirline {
			if request.UserProfile.AllowedAirlines != nil {
				if !contains(request.UserProfile.AllowedAirlines, f.GetIATAAirline()) {
					continue
				}
			}
		}

		// Made it here without being filtered out, so add it to the flights to be returned. The "prune"
		// function removed any message elements that the user is not allowed to see
		returnFlights = append(returnFlights, prune(f, request))
	}

	mapMutex.Unlock()

	sort.Slice(returnFlights, func(i, j int) bool {
		return returnFlights[i].GetSTO().Before(returnFlights[j].GetSTO())
	})

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
			flDup.FlightState.Value = flight.FlightState.Value
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
