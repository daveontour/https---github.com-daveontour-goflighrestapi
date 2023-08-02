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

	defer exeTime(fmt.Sprintf("Get Flight Processing time for %s", c.Request.RequestURI))()
	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")

	if !userProfile.Enabled {
		c.JSON(http.StatusUnauthorized, gin.H{"Error": fmt.Sprintf("%s", "User Access Has Been Disabled")})
		return
	}

	requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI))

	apt := c.Param("apt")
	direction := strings.ToUpper(c.Query("direction"))
	if direction == "" {
		direction = strings.ToUpper(c.Query("d"))
	}
	airline := c.Query("al")
	flt := c.Query("flt")
	if flt == "" {
		flt = c.Query("flight")
	}
	from := c.Query("from")
	to := c.Query("to")
	route := strings.ToUpper(c.Query("route"))
	if route == "" {
		route = c.Query("r")
	}

	response, error := getRequestedFlightsCommon(apt, direction, airline, flt, from, to, route, "", c, nil)
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

	return getRequestedFlightsCommon(apt, direction, airline, "", strconv.Itoa(from), strconv.Itoa(to), route, userToken, nil, qf)

}
func getRequestedFlightsCommon(apt, direction, airline, flt, from, to, route, userToken string, c *gin.Context, qf []ParameterValuePair) (Response, GetFlightsError) {

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

	if apt == "" {
		//apt = userProfile.DefaultAirport

		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Airport not specified"),
		}
	}

	// Check that the requested airport exists in the repository
	if GetRepo(apt) == nil {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New(fmt.Sprintf("Airport %s not found", apt)),
		}
	}

	// Set Default airline if none set
	if airline == "" && userProfile.DefaultAirline != "" {
		airline = userProfile.DefaultAirline
		response.AddWarning(fmt.Sprintf("Airline set to %s by the administration configuration", airline))
	}

	//Check that the user is allowed to access the requested airline
	if !contains(userProfile.AllowedAirports, apt) &&
		!contains(userProfile.AllowedAirports, "*") {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("User is not allowed to access requested airport"),
		}
	}

	response.AirportCode = apt

	// Build the request object
	request := Request{Direction: direction, Airline: airline, FltNum: flt, From: from, To: to, UserProfile: userProfile, Route: route}

	// Reform the request based on the user Profile and the request parameters
	request, response = processCustomFieldQueries(request, response, c, qf)

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
	mapMutex.Lock()
	flights := GetRepo(apt).Flights
	response, err = filterFlights(request, response, flights, c)
	mapMutex.Unlock()

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

func processCustomFieldQueries(request Request, response Response, c *gin.Context, qf []ParameterValuePair) (Request, Response) {

	customFieldQureyMap := make(map[string]string)

	if c != nil {
		// Find the potential customField queries in the request
		queryMap := c.Request.URL.Query()
		for k, v := range queryMap {
			if !contains(reservedParameters, k) {
				customFieldQureyMap[k] = v[0]
			}
		}
	} else if qf != nil {
		for _, pvPair := range qf {
			parameter := pvPair.Parameter
			value := pvPair.Value
			customFieldQureyMap[parameter] = value
		}
	}
	// (even if there are rubbish values still in the request, the GetPropoerty function will handle it

	// Put in new default values
	if request.UserProfile.DefaultQueryableCustomFields != nil {
		for _, pair := range request.UserProfile.DefaultQueryableCustomFields {
			if v, ok := customFieldQureyMap[pair.Parameter]; ok {
				if v != pair.Value {
					response.AddWarning(fmt.Sprintf("Setting query against %s to default value %s", pair.Parameter, pair.Value))
				}
			}
			customFieldQureyMap[pair.Parameter] = pair.Value
		}
	}

	// Remove any queries against unauthorised fields
	remove := []string{}
	for k, _ := range customFieldQureyMap {
		if !contains(request.UserProfile.AllowedCustomFields, k) && !contains(request.UserProfile.AllowedCustomFields, "*") {
			remove = append(remove, k)
		}
	}

	for _, k := range remove {
		delete(customFieldQureyMap, k)
		response.AddWarning(fmt.Sprintf("Ignoring unauthorised query against custom field: %s", k))
	}

	presentQueryableParameters := []ParameterValuePair{}

	for k, v := range customFieldQureyMap {
		presentQueryableParameters = append(presentQueryableParameters, ParameterValuePair{Parameter: k, Value: v})
	}

	request.PresentQueryableParameters = presentQueryableParameters

	return request, response
}
func filterFlights(request Request, response Response, flights map[string]Flight, c *gin.Context) (Response, error) {

	//defer exeTime("Filter, Prune and Sort Flights")()
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

	filterStart := time.Now()
NextFlight:
	for _, f := range flights {

		if f.GetSTO().Before(from) {
			continue
		}

		if f.GetSTO().After(to) {
			continue
		}

		for _, queryableParameter := range request.PresentQueryableParameters {
			queryValue := queryableParameter.Value
			flightValue := f.GetProperty(queryableParameter.Parameter)

			if flightValue == "" || queryValue != flightValue {
				continue NextFlight
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

		if request.FltNum != "" && !strings.Contains(f.GetFlightID(), request.FltNum) {
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
		f.Action = StatusAction
		returnFlights = append(returnFlights, f)
	}

	metricsLogger.Info(fmt.Sprintf("Filter Flights execution time: %s", time.Since(filterStart)))

	returnFlights = prune(returnFlights, request)

	response.NumberOfFlights = len(returnFlights)

	defer exeTime(fmt.Sprintf("Sorting %v Filtered Flights", response.NumberOfFlights))()
	sort.Slice(returnFlights, func(i, j int) bool {
		return returnFlights[i].GetSTO().Before(returnFlights[j].GetSTO())
	})

	response.Flights = returnFlights
	response.CustomFieldQuery = request.PresentQueryableParameters

	return response, nil
}

// Creates a copy of the flight record with the custom fields that the user is allowed to see
func prune(flights []Flight, request Request) (flDups []Flight) {

	defer exeTime(fmt.Sprintf("Pruning %v Filtered Flights", len(flights)))()

	for _, flight := range flights {

		//Go creates a copy with the below assignment
		flDup := flight

		// Clear all the Custom Field Parameters
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

		flDups = append(flDups, flDup)
	}

	return
}
