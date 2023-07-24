package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func startGinServer() {

	gin.SetMode(gin.DebugMode)
	router := gin.New()

	router.POST("/test", testQuery)
	router.GET("/getFlights/:apt", getRequestedFlightsAPI)
	router.GET("/stopJobs/:apt/:userToken", stopJobs)
	router.GET("/stopAllAptJobs/:apt", stopAllAptJobs)
	router.GET("/rescheduleAllAptJobs/:apt", rescheduleAllAptJobs)
	router.GET("/getResources/:apt", getResourceAPI)
	router.GET("/getConfiguredResources/:apt/:resourceType", getConfiguredResources)
	router.GET("/getConfiguredResources/:apt", getConfiguredResources)
	router.GET("/help", func(c *gin.Context) {
		data, err := os.ReadFile("./help.html")
		if err != nil {
			return
		}
		c.Header("Content-Type", "text/html")
		_, _ = c.Writer.Write(data)
	})

	if !serviceConfig.UseHTTPS {

		err := router.Run(serviceConfig.ServiceIPPort)
		if err != nil {
			logger.Fatal("Unable to start HTTP server.")
			wg.Done()
			os.Exit(2)
		}

	} else if serviceConfig.KeyFile != "" && serviceConfig.CertFile != "" {

		server := http.Server{Addr: serviceConfig.ServiceIPPort, Handler: router}
		err := server.ListenAndServeTLS(serviceConfig.CertFile, serviceConfig.KeyFile)
		if err != nil {
			logger.Fatal("Unable to start HTTPS server. Likely cause is that the keyFile or certFile were not found")
			wg.Done()
			os.Exit(2)
		}

	} else {

		cert := &x509.Certificate{
			SerialNumber: big.NewInt(1658),
			Subject: pkix.Name{
				Organization:  []string{"ORGANIZATION_NAME"},
				Country:       []string{"COUNTRY_CODE"},
				Province:      []string{"PROVINCE"},
				Locality:      []string{"CITY"},
				StreetAddress: []string{"ADDRESS"},
				PostalCode:    []string{"POSTAL_CODE"},
			},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(10, 0, 0),
			SubjectKeyId: []byte{1, 2, 3, 4, 6},
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		pub := &priv.PublicKey

		// Sign the certificate
		certificate, _ := x509.CreateCertificate(rand.Reader, cert, cert, pub, priv)

		certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
		keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

		// Generate a key pair from your pem-encoded cert and key ([]byte).
		x509Cert, _ := tls.X509KeyPair(certBytes, keyBytes)

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{x509Cert}}
		server := http.Server{Addr: serviceConfig.ServiceIPPort, Handler: router, TLSConfig: tlsConfig}

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			logger.Fatal("Unable to start HTTPS server with local certificates and key")
			wg.Done()
			os.Exit(2)
		}
	}

}

func stopJobs(c *gin.Context) {
	apt := c.Param("apt")
	userToken := c.Param("userToken")
	s := schedulerMap[apt]
	s.RemoveByTag(userToken)
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s, user %s", apt, userToken))
}

func stopAllAptJobs(c *gin.Context) {
	apt := c.Param("apt")
	s := schedulerMap[apt]
	s.Clear()
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s", apt))
}
func rescheduleAllAptJobs(c *gin.Context) {
	apt := c.Param("apt")
	reloadschedulePushes(apt)
	logger.Info(fmt.Sprintf("Rescheduled All Aiport Jobs Stopped for %s", apt))
}

func getUserProfile(c *gin.Context, userToken string) UserProfile {

	//Read read the file for each request so changes can be made without the need to restart the server

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, "users.json"))

	if err != nil {
		logger.Error("Error Reading " + filepath.Join(exPath, "users.json"))
		logger.Error(err.Error())
		return UserProfile{}
	}

	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var users Users

	json.Unmarshal([]byte(byteResult), &users)

	key := userToken

	if c != nil {
		keys := c.Request.Header["token"]
		key = "default"

		if keys != nil {
			key = keys[0]
		}

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

func testQuery(c *gin.Context) {
	jsonData, _ := ioutil.ReadAll(c.Request.Body)
	fmt.Println(string(jsonData[:]))
}
func getConfiguredResources(c *gin.Context) {
	apt := c.Param("apt")
	resourceType := c.Param("resourceType")

	// Create the response object so we can return early if required
	response := ResourceResponse{}
	c.Writer.Header().Set("Content-Type", "application/json")

	if resourceType != "" {
		response.ResourceType = resourceType
	} else {
		response.ResourceType = "All Resources"
	}

	if resourceType != "" && !strings.Contains("Checkin Gate Stand Carousel Chute", resourceType) {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid resouce type specified. "})
		return
	}

	var err error

	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" && userProfile.DefaultAirport != "" {
		apt = userProfile.DefaultAirport
	}

	//Check that the user is allowed to access the requested airport
	if !contains(userProfile.AllowedAirlines, apt) &&
		!contains(userProfile.AllowedAirlines, "*") {
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

	var alloc = []ConfiguredResourceResponseItem{}

	allocMaps := []map[string]ResourceAllocationMap{
		GetRepo(apt).CheckInAllocationMap,
		GetRepo(apt).GateAllocationMap,
		GetRepo(apt).StandAllocationMap,
		GetRepo(apt).ChuteAllocationMap,
		GetRepo(apt).CarouselAllocationMap}

	for idx, allocMap := range allocMaps {

		//If a resource type has been specified, ignore the rest
		if resourceType != "" {
			if resourceType == "Checkin" && idx != 0 {
				continue
			}
			if resourceType == "Gate" && idx != 1 {
				continue
			}
			if resourceType == "Stand" && idx != 2 {
				continue
			}
			if resourceType == "Chute" && idx != 3 {
				continue
			}
			if resourceType == "Carousel" && idx != 4 {
				continue
			}
		}

		for _, r := range allocMap {

			mapp := allocMap[r.Resource.Name]

			n := ConfiguredResourceResponseItem{
				ResourceTypeCode: mapp.Resource.ResourceTypeCode,
				Name:             mapp.Resource.Name,
				Area:             mapp.Resource.Area,
			}
			alloc = append(alloc, n)

		}
	}

	response.ConfiguredResources = alloc

	// Get the filtered and pruned flights for the request
	//response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	if err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", err.Error())})
	}
}

func getResources(c *gin.Context) {

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

	from := c.Query("from")
	to := c.Query("to")
	updatedSince := c.Query("updatedSince")
	loc, _ := time.LoadLocation("Local")

	// Create the response object so we can return early if required
	response := ResourceResponse{}
	c.Writer.Header().Set("Content-Type", "application/json")

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

	if resourceType != "" && !strings.Contains("Checkin Gate Stand Carousel Chute", resourceType) {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid resouce type specified. "})
		return
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

	updatedSinceTime, updatedSinceErr := time.ParseInLocation("2006-01-02T15:04:05", updatedSince, loc)
	if updatedSinceErr != nil && updatedSince != "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid 'updatedSince' time specified. "})
		return
	}
	var err error

	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" && userProfile.DefaultAirport != "" {
		apt = userProfile.DefaultAirport
	}

	//Check that the user is allowed to access the requested airport
	if !contains(userProfile.AllowedAirlines, apt) &&
		!contains(userProfile.AllowedAirlines, "*") {
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

	var alloc = []AllocationResponseItem{}

	allocMaps := []map[string]ResourceAllocationMap{
		GetRepo(apt).CheckInAllocationMap,
		GetRepo(apt).GateAllocationMap,
		GetRepo(apt).StandAllocationMap,
		GetRepo(apt).ChuteAllocationMap,
		GetRepo(apt).CarouselAllocationMap}

	for idx, allocMap := range allocMaps {

		//If a resource type has been specified, ignore the rest
		if resourceType != "" {
			if strings.ToLower(resourceType) == "checkin" && idx != 0 {
				continue
			}
			if strings.ToLower(resourceType) == "gate" && idx != 1 {
				continue
			}
			if strings.ToLower(resourceType) == "stand" && idx != 2 {
				continue
			}
			if strings.ToLower(resourceType) == "chute" && idx != 3 {
				continue
			}
			if strings.ToLower(resourceType) == "carousel" && idx != 4 {
				continue
			}
		}

		for _, r := range allocMap {

			//If a specific resource has been requested, ignore the rest
			if resource != "" && r.Resource.Name != resource {
				continue
			}

			mapp := allocMap[r.Resource.Name]
			for _, v := range allocMap[r.Resource.Name].FlightAllocationsMap {

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
					continue
				}

				if v.To.Before(fromTime) {
					continue
				}

				if v.From.After(toTime) {
					continue
				}

				if updatedSinceErr == nil {
					if v.LastUpdate.Before(updatedSinceTime) {
						continue
					}
				}

				n := AllocationResponseItem{
					AllocationItem: AllocationItem{From: v.From,
						To:                   v.To,
						FlightID:             v.FlightID,
						Direction:            v.Direction,
						Route:                v.Route,
						AircraftType:         v.AircraftType,
						AircraftRegistration: v.AircraftRegistration,
						LastUpdate:           v.LastUpdate},
					ResourceType: mapp.Resource.ResourceTypeCode,
					Name:         mapp.Resource.Name,
					Area:         mapp.Resource.Area,
				}
				alloc = append(alloc, n)
			}

		}
	}

	sort.Slice(alloc, func(i, j int) bool {
		return alloc[i].From.Before(alloc[j].From)
	})

	response.Allocations = alloc

	// Get the filtered and pruned flights for the request
	//response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	if err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", err.Error())})
	}

}

func getResourceSub(sub UserPushSubscription, userToken string) (ResourceResponse, GetFlightsError) {

	apt := sub.Airport
	flightID := ""
	airline := sub.Airline
	resourceType := sub.ResourceType
	resource := sub.ResourceID
	from := sub.From
	to := sub.To
	updatedSince := ""

	return getResourcesCommon(apt, flightID, airline, resourceType, resource, strconv.Itoa(from), strconv.Itoa(to), updatedSince, userToken, nil)
}

func getResourceAPI(c *gin.Context) {

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

	from := c.Query("from")
	to := c.Query("to")
	updatedSince := c.Query("updatedSince")

	response, error := getResourcesCommon(apt, flightID, airline, resourceType, resource, from, to, updatedSince, "", c)

	// Create the response object so we can return early if required
	c.Writer.Header().Set("Content-Type", "application/json")

	if error.Err == nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", error.Err.Error())})
	}

}

func getResourcesCommon(apt, flightID, airline, resourceType, resource, from, to, updatedSince, userToken string, c *gin.Context) (ResourceResponse, GetFlightsError) {

	loc, _ := time.LoadLocation("Local")

	// Create the response object so we can return early if required
	response := ResourceResponse{}
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

	if resourceType != "" && !strings.Contains("Checkin Gate Stand Carousel Chute", resourceType) {
		return response, GetFlightsError{
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

	updatedSinceTime, updatedSinceErr := time.ParseInLocation("2006-01-02T15:04:05", updatedSince, loc)
	if updatedSinceErr != nil && updatedSince != "" {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("Invalid 'updatedSince' time specified."),
		}
	}

	// Get the profile of the user making the request
	userProfile := getUserProfile(c, userToken)
	response.User = userProfile.UserName

	// Set Default airport if none set
	if apt == "" && userProfile.DefaultAirport != "" {
		apt = userProfile.DefaultAirport
	}

	//Check that the user is allowed to access the requested airport
	if !contains(userProfile.AllowedAirlines, apt) &&
		!contains(userProfile.AllowedAirlines, "*") {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("User is not permitted to access airport"),
		}
	}

	// Check that the requested airport exists in the repository
	//_, ok := repoMap[apt]
	if GetRepo(apt) == nil {
		return response, GetFlightsError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New(fmt.Sprintf("Airport %s not found", apt)),
		}
	}

	response.AirportCode = apt

	var alloc = []AllocationResponseItem{}

	allocMaps := []map[string]ResourceAllocationMap{
		GetRepo(apt).CheckInAllocationMap,
		GetRepo(apt).GateAllocationMap,
		GetRepo(apt).StandAllocationMap,
		GetRepo(apt).ChuteAllocationMap,
		GetRepo(apt).CarouselAllocationMap}

	for idx, allocMap := range allocMaps {

		//If a resource type has been specified, ignore the rest
		if resourceType != "" {
			if strings.ToLower(resourceType) == "checkin" && idx != 0 {
				continue
			}
			if strings.ToLower(resourceType) == "gate" && idx != 1 {
				continue
			}
			if strings.ToLower(resourceType) == "stand" && idx != 2 {
				continue
			}
			if strings.ToLower(resourceType) == "chute" && idx != 3 {
				continue
			}
			if strings.ToLower(resourceType) == "carousel" && idx != 4 {
				continue
			}
		}

		for _, r := range allocMap {

			//If a specific resource has been requested, ignore the rest
			if resource != "" && r.Resource.Name != resource {
				continue
			}

			mapp := allocMap[r.Resource.Name]
			for _, v := range allocMap[r.Resource.Name].FlightAllocationsMap {

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
					continue
				}

				if v.To.Before(fromTime) {
					continue
				}

				if v.From.After(toTime) {
					continue
				}

				if updatedSinceErr == nil {
					if v.LastUpdate.Before(updatedSinceTime) {
						continue
					}
				}

				n := AllocationResponseItem{
					AllocationItem: AllocationItem{From: v.From,
						To:                   v.To,
						FlightID:             v.FlightID,
						Direction:            v.Direction,
						Route:                v.Route,
						AircraftType:         v.AircraftType,
						AircraftRegistration: v.AircraftRegistration,
						LastUpdate:           v.LastUpdate},
					ResourceType: mapp.Resource.ResourceTypeCode,
					Name:         mapp.Resource.Name,
					Area:         mapp.Resource.Area,
				}
				alloc = append(alloc, n)
			}

		}
	}

	sort.Slice(alloc, func(i, j int) bool {
		return alloc[i].From.Before(alloc[j].From)
	})

	response.Allocations = alloc

	// Get the filtered and pruned flights for the request
	//response, err = filterFlights(request, response, repoMap[apt].Flights, c)

	return response, GetFlightsError{
		StatusCode: http.StatusOK,
		Err:        nil,
	}
}

type GetFlightsError struct {
	StatusCode int
	Err        error
}

func (r *GetFlightsError) Error() string {
	return fmt.Sprintf("status %d: err %v", r.StatusCode, r.Err)
}

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

	response, error := getRequestedFlightsCommon(apt, direction, airline, from, to, route, "", c)

	// Create the response object so we can return early if required
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

	return getRequestedFlightsCommon(apt, direction, airline, strconv.Itoa(from), strconv.Itoa(to), route, userToken, nil)

}
func getRequestedFlightsCommon(apt, direction, airline, from, to, route, userToken string, c *gin.Context) (Response, GetFlightsError) {

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
	request, response = normaliseRequest(request, response, c)

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

func normaliseRequest(request Request, response Response, c *gin.Context) (Request, Response) {
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
	loc, _ := time.LoadLocation("Local")

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

func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
