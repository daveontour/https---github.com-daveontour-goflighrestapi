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

func GetResourceSub(sub models.UserPushSubscription, userToken string) (models.ResourceResponse, models.GetFlightsError) {

	apt := sub.Airport
	flightID := ""
	airline := sub.Airline
	resourceType := sub.ResourceType
	resource := sub.ResourceID
	from := sub.From
	to := sub.To
	updatedSince := ""
	sortBy := "time"

	return getResourcesCommon(apt, flightID, airline, resourceType, resource, strconv.Itoa(from), strconv.Itoa(to), updatedSince, sortBy, userToken, nil)
}

func GetResourceAPI(c *gin.Context) {

	defer globals.ExeTime(fmt.Sprintf("Get Resources Request for %s", c.Request.URL))()
	// Get the profile of the user making the request
	userProfile := GetUserProfile(c, "")

	if !userProfile.Enabled {
		c.JSON(http.StatusUnauthorized, gin.H{"Error": fmt.Sprintf("%s", "User Access Has Been Disabled")})
		return
	}

	globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI))

	apt := c.Param("apt")

	flightID := c.Query("flight")
	if flightID == "" {
		flightID = c.Query("flt")
	}

	airline := c.Query("airline")
	if airline == "" {
		airline = c.Query("al")
	}
	resourceType := c.Query("resourceType")
	if resourceType == "" {
		resourceType = c.Query("rt")
	}

	resource := c.Query("resource")
	if resource == "" {
		resource = c.Query("id")
	}
	sortBy := c.Query("sort")
	if sortBy == "" {
		sortBy = "resource"
	}

	from := c.Query("from")
	to := c.Query("to")
	updatedSince := c.Query("updatedSince")

	response, error := getResourcesCommon(apt, flightID, airline, resourceType, resource, from, to, updatedSince, sortBy, "", c)

	fileName, err := writeResourceResponseToFile(response, &userProfile)

	// defer func() {
	// 	globals.FileDeleteChannel <- fileName
	// }()

	if err == nil {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.File(fileName)
		defer func() {
			err := os.Remove(fileName)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println("Temp file deleted")
			}
		}()
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", error.Error())})
	}
}

func getResourcesCommon(apt, flightID, airline, resourceType, resource, from, to, updatedSince, sortBy, userToken string, c *gin.Context) (models.ResourceResponse, models.GetFlightsError) {

	response := models.ResourceResponse{}
	//	c.Writer.Header().Set("Content-Type", "application/json")

	if resourceType != "" {
		response.ResourceType = resourceType
	} else {
		response.ResourceType = "All Resource Types"
	}

	if resource != "" {
		response.ResourceID = resource
	} else {
		response.ResourceID = "All"
	}

	if flightID != "" {
		response.FlightID = flightID
	} else {
		response.FlightID = "All Flights"
	}

	if airline != "" {
		response.Airline = airline
	} else {
		response.Airline = "All Airlines"
	}

	if resourceType != "" && !strings.Contains(strings.ToLower("Checkin Gate Stand Carousel Chute Checkins Gates Stands Carousels Chutes"), strings.ToLower(resourceType)) {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Invalid resouce type specified."),
		}
	}

	fromOffset, fromErr := strconv.Atoi(from)
	if fromErr != nil {
		fromOffset = -12
	}

	fromTime := time.Now().Add(time.Hour * time.Duration(fromOffset))
	response.FromResource = fromTime.Format("2006-01-02T15:04:05")

	toOffset, toErr := strconv.Atoi(to)
	if toErr != nil {
		toOffset = 24
	}

	toTime := time.Now().Add(time.Hour * time.Duration(toOffset))
	response.ToResource = toTime.Format("2006-01-02T15:04:05")

	updatedSinceTime, updatedSinceErr := time.ParseInLocation("2006-01-02T15:04:05", updatedSince, timeservice.Loc)
	if updatedSinceErr != nil && updatedSince != "" {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Invalid 'updatedSince' time specified."),
		}
	}

	// Get the profile of the user making the request
	userProfile := GetUserProfile(c, userToken)
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" {
		//apt = userProfile.DefaultAirport
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Airport not specified"),
		}
	}

	//Check that the user is allowed to access the requested airport
	if !globals.Contains(userProfile.AllowedAirports, apt) &&
		!globals.Contains(userProfile.AllowedAirports, "*") {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("User is not permitted to access airport"),
		}
	}

	// Check that the requested airport exists in the repository
	//_, ok := repoMap[apt]
	if GetRepo(apt) == nil {
		return response, models.GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New(fmt.Sprintf("Airport %s not found", apt)),
		}
	}

	response.AirportCode = apt

	var alloc = []models.AllocationResponseItem{}

	globals.MapMutex.Lock()
	defer globals.MapMutex.Unlock()

	repo := GetRepo(apt)
	allocMaps := []models.ResourceLinkedList{
		repo.CheckInList,
		repo.GateList,
		repo.StandList,
		repo.ChuteList,
		repo.CarouselList}

	filterStart := time.Now()
	for idx, allocMap := range allocMaps {

		//If a resource type has been specified, ignore the rest
		if resourceType != "" {
			if (strings.ToLower(resourceType) == "checkin" || strings.ToLower(resourceType) == "checkins") && idx != 0 {
				continue
			}
			if (strings.ToLower(resourceType) == "gate" || strings.ToLower(resourceType) == "gates") && idx != 1 {
				continue
			}
			if (strings.ToLower(resourceType) == "stand" || strings.ToLower(resourceType) == "stands") && idx != 2 {
				continue
			}
			if (strings.ToLower(resourceType) == "chute" || strings.ToLower(resourceType) == "chutes") && idx != 3 {
				continue
			}
			if (strings.ToLower(resourceType) == "carousel" || strings.ToLower(resourceType) == "carousels") && idx != 4 {
				continue
			}
		}

		r := allocMap.Head
		for r != nil {

			//If a specific resource has been requested, ignore the rest
			if resource != "" && r.Resource.Name != resource {
				r = r.NextNode
				continue
			}

			list := r.FlightAllocationsList

			v := list.Head

			for v != nil {

				test := false

				if airline != "" && strings.HasPrefix(v.FlightID, airline) {
					test = true
				}
				if flightID != "" && strings.Contains(v.FlightID, flightID) {
					test = true
				}

				if airline == "" && flightID == "" {
					test = true
				}

				if !test {
					v = v.NextNode
					continue
				}

				if v.To.Before(fromTime) {
					v = v.NextNode
					continue
				}

				if v.From.After(toTime) {
					v = v.NextNode
					continue
				}

				if updatedSinceErr == nil {
					if v.LastUpdate.Before(updatedSinceTime) {
						v = v.NextNode
						continue
					}
				}

				n := models.AllocationResponseItem{
					AllocationItem: models.AllocationItem{From: v.From,
						To:                   v.To,
						FlightID:             v.FlightID,
						Direction:            v.Direction,
						Route:                v.Route,
						AircraftType:         v.AircraftType,
						AircraftRegistration: v.AircraftRegistration,
						LastUpdate:           v.LastUpdate},
					ResourceType: r.Resource.ResourceTypeCode,
					Name:         r.Resource.Name,
					Area:         r.Resource.Area,
				}
				alloc = append(alloc, n)
				v = v.NextNode
			}
			r = r.NextNode
		}
	}

	globals.MetricsLogger.Info(fmt.Sprintf("Filter Resources execution time: %s", time.Since(filterStart)))

	sortStart := time.Now()
	if strings.ToLower(sortBy) == "time" {
		sort.Slice(alloc, func(i, j int) bool {
			return alloc[i].From.Before(alloc[j].From)
		})
	} else {
		sort.Slice(alloc, func(i, j int) bool {
			si := alloc[i].ResourceType + alloc[i].Name
			sj := alloc[j].ResourceType + alloc[j].Name

			r := strings.Compare(si, sj)

			if r < 1 {
				return true
			} else {
				return false
			}
		})
	}

	globals.MetricsLogger.Info(fmt.Sprintf("Sort Resources execution time: %s", time.Since(sortStart)))

	response.Allocations = alloc

	// Get the filtered and pruned flights for the request
	//response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	return response, models.GetFlightsError{
		StatusCode: http.StatusOK,
		Err:        nil,
	}
}

func GetConfiguredResources(c *gin.Context) {

	defer globals.ExeTime(fmt.Sprintf("Get Configured Resources Request for %s", c.Request.URL))()
	// Get the profile of the user making the request
	userProfile := GetUserProfile(c, "")
	if !userProfile.Enabled {
		c.JSON(http.StatusUnauthorized, gin.H{"Error": fmt.Sprintf("%s", "User Access Has Been Disabled")})
		return
	}
	globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI))

	apt := c.Param("apt")
	resourceType := c.Param("resourceType")

	// Create the response object so we can return early if required
	response := models.ResourceResponse{}
	c.Writer.Header().Set("Content-Type", "application/json")

	if resourceType != "" {
		response.ResourceType = resourceType
	} else {
		response.ResourceType = "All Resources"
	}

	if resourceType != "" && !strings.Contains(strings.ToLower("Checkin Gate Stand Carousel Chute Checkins Gates Stands Carousels Chutes"), strings.ToLower(resourceType)) {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid resouce type specified. "})
		return
	}

	var err error

	// Get the profile of the user making the request
	//userProfile := getUserProfile(c, "")
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Airport not specified %s"})
		return
	}

	//Check that the user is allowed to access the requested airport
	if !globals.Contains(userProfile.AllowedAirlines, apt) &&
		!globals.Contains(userProfile.AllowedAirlines, "*") {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "User is not permitted to access airport %s"})
		return
	}

	// Check that the requested airport exists in the repository
	//_, ok := repoMap[apt]
	if GetRepo(apt) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Airport %s not found", apt)})
		return
	}

	response.AirportCode = apt

	var alloc = []models.ConfiguredResourceResponseItem{}

	repo := GetRepo(apt)
	allocMaps := []models.ResourceLinkedList{
		repo.CheckInList,
		repo.GateList,
		repo.StandList,
		repo.ChuteList,
		repo.CarouselList}

	for idx, allocMap := range allocMaps {

		//If a resource type has been specified, ignore the rest
		if resourceType != "" {
			if (strings.ToLower(resourceType) == "checkin" || strings.ToLower(resourceType) == "checkins") && idx != 0 {
				continue
			}
			if (strings.ToLower(resourceType) == "gate" || strings.ToLower(resourceType) == "gates") && idx != 1 {
				continue
			}
			if (strings.ToLower(resourceType) == "stand" || strings.ToLower(resourceType) == "stands") && idx != 2 {
				continue
			}
			if (strings.ToLower(resourceType) == "chute" || strings.ToLower(resourceType) == "chutes") && idx != 3 {
				continue
			}
			if (strings.ToLower(resourceType) == "carousel" || strings.ToLower(resourceType) == "carousels") && idx != 4 {
				continue
			}
		}

		struc := allocMap.Head

		for struc != nil {

			n := models.ConfiguredResourceResponseItem{
				ResourceTypeCode: struc.Resource.ResourceTypeCode,
				Name:             struc.Resource.Name,
				Area:             struc.Resource.Area,
			}
			alloc = append(alloc, n)
			struc = struc.NextNode
		}
	}

	response.ConfiguredResources = alloc

	// Get the filtered and pruned flights for the request
	//response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	file, errs := os.CreateTemp("", "getresourcettemp-*.txt")
	if errs != nil {
		fmt.Println(errs)
		return
	}
	fwb := bufio.NewWriterSize(file, 32768)
	defer os.Remove(file.Name())

	error := response.WriteJSON(fwb)
	error2 := fwb.WriteByte('}')
	error3 := fwb.Flush()

	if error == nil && error2 == nil && error3 == nil {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.File(file.Name())
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", error.Error())})
	}

	if err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", err.Error())})
	}
}

func writeResourceResponseToFile(response models.ResourceResponse, userProfile *models.UserProfile) (fileName string, e error) {

	file, errs := os.CreateTemp("", "getresourcettemp-*.txt")
	if errs != nil {
		fmt.Println(errs)
		return
	}
	defer file.Close()

	// Create the response object so we can return early if required
	fmt.Println("Temporary file created : ", file.Name())

	fwb := bufio.NewWriterSize(file, 32768)
	defer func() {
		globals.FileDeleteChannel <- file.Name()
	}()

	response.WriteJSON(fwb)
	fwb.Flush()

	return fileName, nil
}
