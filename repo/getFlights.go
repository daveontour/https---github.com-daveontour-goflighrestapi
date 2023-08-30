package repo

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/models"
	"flightresourcerestapi/timeservice"

	"github.com/gin-gonic/gin"
)

func GetUserProfile(c *gin.Context, userToken string) models.UserProfile {

	defer globals.ExeTime("Getting User Profile")()

	key := userToken

	if c != nil {
		keys := c.Request.Header["Token"]
		key = "default"

		if keys != nil {
			key = keys[0]
		}

	}
	users := globals.GetUserProfiles()
	userProfile := models.UserProfile{}

	for _, u := range users {
		if key == u.Key {
			userProfile = u
			break
		}
	}

	return userProfile

}

func GetRequestedFlightsAPI(c *gin.Context) {
	defer globals.ExeTime(fmt.Sprintf("Get Flight Processing time for %s", c.Request.RequestURI))()

	userProfile := GetUserProfile(c, "")

	if !userProfile.Enabled {
		c.JSON(http.StatusUnauthorized, gin.H{"Error": "User Access Has Been Disabled"})
		return
	}

	globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI))

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

	response, _ := GetRequestedFlightsCommon(apt, direction, airline, flt, from, to, route, "", c, nil)

	fileName, err := writeFlightResponseToFile(response, &userProfile)

	if err == nil {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.File(fileName)

		// f, _ := os.OpenFile(fileName, os.O_RDONLY, 0777)
		// fi, err := f.Stat()
		// if err != nil {
		// 	// Could not obtain stat, handle error
		// }

		// c.DataFromReader(200, fi.Size(), "application/json", f, nil)

		defer func() {
			// err = f.Close()
			// if err != nil {
			// 	fmt.Println(err.Error())
			// } else {
			err := os.Remove(fileName)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println("Temp file deleted")
			}
			//		}
		}()
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
	}
}

func GetRequestedFlightsSub(sub models.UserPushSubscription, userToken string) (models.Response, models.GetFlightsError) {
	apt := sub.Airport
	direction := strings.ToUpper(sub.Direction)
	airline := sub.Airline
	from := sub.From
	to := sub.To
	route := strings.ToUpper(sub.Route)
	qf := sub.QueryableCustomFields

	return GetRequestedFlightsCommon(apt, direction, airline, "", strconv.Itoa(from), strconv.Itoa(to), route, userToken, nil, qf)

}
func GetRequestedFlightsCommon(apt, direction, airline, flt, from, to, route, userToken string, c *gin.Context, qf []models.ParameterValuePair) (models.Response, models.GetFlightsError) {

	// Create the response object so we can return early if required
	response := models.Response{}

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
	userProfile := GetUserProfile(c, userToken)
	response.User = userProfile.UserName

	if apt == "" {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Airport not specified"),
		}
	}

	// Check that the requested airport exists in the repository
	if GetRepo(apt) == nil {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New(fmt.Sprintf("Airport %s not found", apt)),
		}
	}

	// Set Default airline if none set
	if airline == "" && userProfile.DefaultAirline != "" {
		airline = userProfile.DefaultAirline
		response.AddWarning(fmt.Sprintf("Airline set to %s by the administration configuration", airline))
	} else {
		response.Airline = airline
	}

	if flt != "" {
		response.Flight = flt
	}

	//Check that the user is allowed to access the requested airport
	if !globals.Contains(userProfile.AllowedAirports, apt) &&
		!globals.Contains(userProfile.AllowedAirports, "*") {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("User is not allowed to access requested airport"),
		}
	}

	response.AirportCode = apt

	// Build the request object
	request := models.Request{Direction: direction, Airline: airline, FltNum: flt, From: from, To: to, UserProfile: userProfile, Route: route}

	// Reform the request based on the user Profile and the request parameters
	request, response = processCustomFieldQueries(request, response, c, qf)

	// If the user is requesting a particular airline, check that they are allowed to access that airline
	if airline != "" && userProfile.AllowedAirlines != nil {
		if !globals.Contains(userProfile.AllowedAirlines, airline) &&
			!globals.Contains(userProfile.AllowedAirlines, "*") {
			return response, models.GetFlightsError{
				StatusCode: http.StatusBadRequest,
				Err:        errors.New("unavailable"),
			}
		}
	}

	var err error

	// Get the filtered and pruned flights for the request
	globals.MapMutex.Lock()
	flights := GetRepo(apt).FlightLinkedList
	response, err = filterFlights(request, response, flights, c, GetRepo(apt))
	globals.MapMutex.Unlock()

	if err == nil {
		return response, models.GetFlightsError{
			StatusCode: http.StatusOK,
			Err:        nil,
		}
	} else {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        err,
		}
	}
}

func processCustomFieldQueries(request models.Request, response models.Response, c *gin.Context, qf []models.ParameterValuePair) (models.Request, models.Response) {

	customFieldQureyMap := make(map[string]string)

	if c != nil {
		// Find the potential customField queries in the request
		queryMap := c.Request.URL.Query()
		for k, v := range queryMap {
			if !globals.Contains(globals.ReservedParameters, k) {
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
	for k := range customFieldQureyMap {
		if !globals.Contains(request.UserProfile.AllowedCustomFields, k) && !globals.Contains(request.UserProfile.AllowedCustomFields, "*") {
			remove = append(remove, k)
		}
	}

	for _, k := range remove {
		delete(customFieldQureyMap, k)
		response.AddWarning(fmt.Sprintf("Ignoring unauthorised query against custom field: %s", k))
	}

	presentQueryableParameters := []models.ParameterValuePair{}

	for k, v := range customFieldQureyMap {
		presentQueryableParameters = append(presentQueryableParameters, models.ParameterValuePair{Parameter: k, Value: v})
	}

	request.PresentQueryableParameters = presentQueryableParameters

	return request, response
}
func filterFlights(request models.Request, response models.Response, flightsLinkedList models.FlightLinkedList, c *gin.Context, repo *models.Repository) (models.Response, error) {

	//defer exeTime("Filter, Prune and Sort Flights")()
	//returnFlights := []models.Flight{}

	// var from time.Time
	// var to time.Time
	var updatedSinceTime time.Time

	fromOffset, fromErr := strconv.Atoi(request.From)
	if fromErr != nil {
		fromOffset = -12
	}

	from := time.Now().Add(time.Hour * time.Duration(fromOffset))

	if from.Before(repo.CurrentLowerLimit) {
		from = repo.CurrentLowerLimit
		response.AddWarning(fmt.Sprintf("Requested lower time limit outside cache boundaries. Set to %s", from.Format("2006-01-02T15:04:05")))
	}
	response.From = from.Format("2006-01-02T15:04:05")

	toOffset, toErr := strconv.Atoi(request.To)
	if toErr != nil {
		toOffset = 24
	}

	to := time.Now().Add(time.Hour * time.Duration(toOffset))
	if to.After(repo.CurrentUpperLimit) {
		to = repo.CurrentUpperLimit
		response.AddWarning(fmt.Sprintf("Requested upper time limit outside cache boundaries. Set to %s", to.Format("2006-01-02T15:04:05")))
	}

	response.To = to.Format("2006-01-02T15:04:05")

	if request.UpdatedSince != "" {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", request.UpdatedSince, timeservice.Loc)
		if err != nil {
			return models.Response{}, err
		} else {
			updatedSinceTime = t
			response.To = t.String()
		}
	}

	allowedAllAirline := false
	if request.UserProfile.AllowedAirlines != nil {
		if globals.Contains(request.UserProfile.AllowedAirlines, "*") {
			allowedAllAirline = true
		}
	}

	filterStart := time.Now()

	currentFlight := flightsLinkedList.Head

NextFlight:
	for currentFlight != nil {

		if currentFlight.GetSTO().Before(from) {
			currentFlight = currentFlight.NextNode
			continue
		}

		if currentFlight.GetSTO().After(to) {
			currentFlight = currentFlight.NextNode
			continue
		}

		for _, queryableParameter := range request.PresentQueryableParameters {
			queryValue := queryableParameter.Value
			flightValue := currentFlight.GetProperty(queryableParameter.Parameter)

			if flightValue == "" || queryValue != flightValue {
				currentFlight = currentFlight.NextNode
				continue NextFlight
			}
		}

		// Flight direction filter
		if strings.HasPrefix(request.Direction, "D") && currentFlight.IsArrival() {
			currentFlight = currentFlight.NextNode
			continue
		}
		if strings.HasPrefix(request.Direction, "A") && !currentFlight.IsArrival() {
			currentFlight = currentFlight.NextNode
			continue
		}

		// Requested Airline Code filter
		if request.Airline != "" && currentFlight.GetIATAAirline() != request.Airline {
			currentFlight = currentFlight.NextNode
			continue
		}

		// RequestedRoute filter
		if request.Route != "" && !strings.Contains(currentFlight.GetFlightRoute(), request.Route) {
			currentFlight = currentFlight.NextNode
			continue
		}

		if request.FltNum != "" && !strings.Contains(currentFlight.GetFlightID(), request.FltNum) {
			currentFlight = currentFlight.NextNode
			continue
		}

		if request.UpdatedSince != "" {
			if currentFlight.LastUpdate.Before(updatedSinceTime) {
				currentFlight = currentFlight.NextNode
				continue
			}
		}

		// Filter out airlines that the user is not allowed to see
		// "*" entry in AllowedAirlines allows all.
		if !allowedAllAirline {
			if request.UserProfile.AllowedAirlines != nil {
				if !globals.Contains(request.UserProfile.AllowedAirlines, currentFlight.GetIATAAirline()) {
					currentFlight = currentFlight.NextNode
					continue
				}
			}
		}

		// Made it here without being filtered out, so add it to the flights to be returned.
		currentFlight.Action = globals.StatusAction
		//	response.ResponseFlights.AddNode(models.FlightResponseItem{FlightPtr: currentFlight})
		response.ResponseFlights = append(response.ResponseFlights, models.FlightResponseItem{FlightPtr: currentFlight, STO: currentFlight.GetSTO()})

		currentFlight = currentFlight.NextNode
	}

	globals.MetricsLogger.Info(fmt.Sprintf("Filter Flights execution time: %s", time.Since(filterStart)))

	//***Important
	//Pruning is now done at the output to avoid creating additional copies of the data structure

	response.NumberOfFlights = len(response.ResponseFlights)

	defer globals.ExeTime(fmt.Sprintf("Sorting %v Filtered Flights", response.NumberOfFlights))()
	sort.Slice(response.ResponseFlights, func(i, j int) bool {
		return response.ResponseFlights[i].STO.Before(response.ResponseFlights[j].STO)
	})

	response.CustomFieldQuery = request.PresentQueryableParameters

	return response, nil
}

func writeFlightResponseToFile(response models.Response, userProfile *models.UserProfile) (fileName string, e error) {

	file, errs := os.CreateTemp("", "getflighttemp-*.txt")
	if errs != nil {
		fmt.Println(errs)
		return
	}
	fwb := bufio.NewWriterSize(file, 32768)
	defer file.Close()

	fmt.Println("The temporary file is created:", file.Name())
	fwb.WriteByte('{')
	fwb.WriteString("\"Airport\":\"" + response.AirportCode + "\",")
	fwb.WriteString("\"Direction\":\"" + response.Direction + "\",")
	fwb.WriteString("\"ScheduleFlightsFrom\":\"" + response.From + "\",")
	fwb.WriteString("\"ScheduleFlightsTo\":\"" + response.To + "\",")
	fwb.WriteString("\"NumberOfFlights\":\"" + fmt.Sprintf("%v", response.NumberOfFlights) + "\",")
	if response.Airline != "" {
		fwb.WriteString("\"Airline\":\"" + response.Airline + "\",")
	} else {
		fwb.WriteString("\"Airline\":\"*\",")
	}
	if response.Flight != "" {
		fwb.WriteString("\"Flight\":\"" + response.Flight + "\",")
	} else {
		fwb.WriteString("\"Flight\":\"*\",")
	}
	if response.Route != "" {
		fwb.WriteString("\"Route\":\"" + response.Route + "\",")
	} else {
		fwb.WriteString("\"Route\":\"*\",")
	}
	fwb.WriteString("\"CustomFieldQuery\":[")
	for idx, w := range response.CustomFieldQuery {
		if idx > 0 {
			fwb.WriteString(",")
		}
		fwb.WriteString("{\"Parameter\":\"" + w.Parameter + "\",\"Value\":\"" + w.Value + "\"}")
	}
	fwb.WriteString("],")

	fwb.WriteString("\"Warnings\":[")
	for idx, w := range response.Warnings {
		if idx > 0 {
			fwb.WriteString(",")
		}
		fwb.WriteString("\"" + w + "\"")
	}
	fwb.WriteString("],")

	fwb.WriteString("\"Errors\":[")
	for idx, w := range response.Errors {
		if idx > 0 {
			fwb.WriteString(",")
		}
		fwb.WriteString("\"" + w + "\"")
	}
	fwb.WriteString("],")

	err := models.WriteFlightsInJSON(fwb, response.ResponseFlights, userProfile)
	err2 := fwb.WriteByte('}')
	err3 := fwb.Flush()

	if err == nil && err2 == nil && err3 == nil {
		return file.Name(), nil
	} else {
		return "", errors.New("error creating response file")
	}

}

// Creates a copy of the flight record with the custom fields that the user is allowed to see
// func prune(flights []models.Flight, request models.Request) (flDups []models.Flight) {

// 	defer globals.ExeTime(fmt.Sprintf("Pruning %v Filtered Flights", len(flights)))()

// 	for _, flight := range flights {

// 		//Go creates a copy with the below assignment. The assignment cretaes a new copy of the struct, so the original is left untouched
// 		flDup := flight

// 		// Clear all the Custom Field Parameters
// 		flDup.FlightState.Values = []models.Value{}

// 		// If Allowed CustomFields is not nil, then filter the custome fields
// 		// if "*" in list then it is all custom fields
// 		// Extra safety, if the parameter is not defined, then no results returned

// 		if request.UserProfile.AllowedCustomFields != nil {
// 			if globals.Contains(request.UserProfile.AllowedCustomFields, "*") {
// 				flDup.FlightState.Values = flight.FlightState.Values
// 			} else {
// 				for _, property := range request.UserProfile.AllowedCustomFields {
// 					data := flight.GetProperty(property)

// 					if data != "" {
// 						flDup.FlightState.Values = append(flDup.FlightState.Values, models.Value{PropertyName: property, Text: data})
// 					}
// 				}
// 			}
// 		}

// 		changes := []models.Change{}

// 		for ii := 0; ii < len(flDup.FlightChanges.Changes); ii++ {
// 			ok := globals.Contains(request.UserProfile.AllowedCustomFields, flDup.FlightChanges.Changes[ii].PropertyName)
// 			ok = ok || request.UserProfile.AllowedCustomFields == nil
// 			if ok {
// 				changes = append(changes, flDup.FlightChanges.Changes[ii])
// 			}
// 		}

// 		flDup.FlightChanges.Changes = changes

// 		flDups = append(flDups, flDup)
// 	}

// 	return
// }
