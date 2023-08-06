package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/jandauz/go-msmq"
)

const xmlBody = `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ams6="http://www.sita.aero/ams6-xml-api-webservice">
<soapenv:Header/>
<soapenv:Body>
   <ams6:GetFlights>
	  <!--Optional:-->
	  <ams6:sessionToken>%s</ams6:sessionToken>
	  <!--Optional:-->
	  <ams6:from>%sT00:00:00</ams6:from>
	  <!--Optional:-->
	  <ams6:to>%sT00:00:00</ams6:to> 
	  <!--Optional:-->
	  <ams6:airport>%s</ams6:airport>
	  <!--Optional:-->
   </ams6:GetFlights>
</soapenv:Body>
</soapenv:Envelope>`

const testNativeAPIMessage = `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ams6="http://www.sita.aero/ams6-xml-api-webservice">
<soapenv:Header/>
<soapenv:Body>
   <ams6:GetAirports>
	  <!--Optional:-->
	  <ams6:sessionToken>%sf</ams6:sessionToken>
   </ams6:GetAirports>
</soapenv:Body>
</soapenv:Envelope>`

func GetRepo(airportCode string) *Repository {
	for idx, repo := range repoList {
		if repo.AMSAirport == airportCode {
			return &repoList[idx]
		}
	}
	return nil
}

func InitRepositories() {

	var repos Repositories

	err := airportsViper.Unmarshal(&repos)
	if err != nil {
		fmt.Println(err)
	}

	for _, v := range repos.Repositories {
		repoList = append(repoList, v)
		go initRepository(v.AMSAirport)
	}
}

func reInitAirport(aptCode string) {

	var repos Repositories
	airportsViper.ReadInConfig()
	airportsViper.Unmarshal(&repos)

	for _, v := range repos.Repositories {
		if v.AMSAirport != aptCode {
			continue
		}
		repoList = append(repoList, v)
	}

	s := refreshSchedulerMap[aptCode]
	if s != nil {
		s.Clear()
	}

	go initRepository(aptCode)

}

func initRepository(airportCode string) {

	defer exeTime(fmt.Sprintf("Initialising Repository for %s", airportCode))()

	//Make sure the required services are available
	for !testNativeAPIConnectivity(airportCode) || !testRestAPIConnectivity(airportCode) {
		logger.Warn(fmt.Sprintf("AMS Webservice API or AMS RestAPI not avaiable for %s. Will try again in 8 seconds", airportCode))
		time.Sleep(8 * time.Second)
	}

	// Purge the listening queue first before doing the Initializarion of the repository
	opts := []msmq.QueueInfoOption{
		msmq.WithPathName(GetRepo(airportCode).NotificationListenerQueue),
	}
	queueInfo, err := msmq.NewQueueInfo(opts...)
	if err != nil {
		log.Fatal(err)
	}

	queue, err := queueInfo.Open(msmq.Receive, msmq.DenyNone)

	if err == nil {
		purgeErr := queue.Purge()
		if purgeErr != nil {
			if isDebug {
				logger.Error("Error purging listening queue")
			}
		} else {
			if isDebug {
				logger.Info("Listening queue purged OK")
			}
		}
	}

	populateResourceMaps(airportCode)

	// The Maintence job schedules a repository population which inits the system
	go maintainRepository(airportCode)
}

func populateResourceMaps(airportCode string) {

	repo := GetRepo(airportCode)
	logger.Info(fmt.Sprintf("Populating Resource Maps for %s", airportCode))
	// Retrieve the available resources

	var checkIns FixedResources
	xml.Unmarshal(getResource(airportCode, "CheckIns"), &checkIns)
	(*repo).CheckInList.AddNodes(checkIns.Values)

	var stands FixedResources
	xml.Unmarshal(getResource(airportCode, "Stands"), &stands)
	(*repo).StandList.AddNodes(stands.Values)

	var gates FixedResources
	xml.Unmarshal(getResource(airportCode, "Gates"), &gates)
	(*repo).GateList.AddNodes(gates.Values)

	var carousels FixedResources
	xml.Unmarshal(getResource(airportCode, "Carousels"), &carousels)
	(*repo).CarouselList.AddNodes(carousels.Values)

	var chutes FixedResources
	xml.Unmarshal(getResource(airportCode, "Chutes"), &chutes)
	(*repo).ChuteList.AddNodes(chutes.Values)

	logger.Info(fmt.Sprintf("Completed Populating Resource Maps for %s", airportCode))
}

func maintainRepository(airportCode string) {

	// Schedule the regular refresh
	go scheduleUpdates(airportCode)

	//Listen to the notification queue
	opts := []msmq.QueueInfoOption{
		msmq.WithPathName(GetRepo(airportCode).NotificationListenerQueue),
	}
	queueInfo, err := msmq.NewQueueInfo(opts...)
	if err != nil {
		log.Fatal(err)
	}

Reconnect:
	for {

		queue, err := queueInfo.Open(msmq.Receive, msmq.DenyNone)
		if err != nil {
			logger.Error(err)
			continue Reconnect
		}

		for {

			msg, err := queue.Receive() //This call blocks until a message is available
			if err != nil {
				logger.Error(err)
				continue Reconnect
			}

			message, _ := msg.Body()

			logger.Debug(fmt.Sprintf("Received Message length %d\n", len(message)))

			if strings.Contains(message, "FlightUpdatedNotification") {
				go updateFlightEntry(message)
				continue
			}
			if strings.Contains(message, "FlightCreatedNotification") {
				go createFlightEntry(message)
				continue
			}
			if strings.Contains(message, "FlightDeletedNotification") {
				go deleteFlightEntry(message)
				continue
			}
		}
	}
}
func scheduleUpdates(airportCode string) {

	// Schedule the regular refresh

	today := time.Now().Format("2006-01-02")
	startTimeStr := today + "T" + configViper.GetString("ScheduleUpdateJob")
	startTime, _ := time.ParseInLocation("2006-01-02T15:04:05", startTimeStr, loc)

	s := gocron.NewScheduler(time.Local)

	refreshSchedulerMap[airportCode] = s

	// Schedule the regular update of the repositoiry
	s.Every(configViper.GetString("ScheduleUpdateJobIntervalInHours")).Hours().StartAt(startTime).Do(func() { updateRepository(airportCode) })

	// Kick off an intial load on startup
	s.Every(1).Millisecond().LimitRunsTo(1).Do(func() { loadRepositoryOnStartup(airportCode) })

	logger.Info(fmt.Sprintf("Regular updates of the repository have been scheduled at %s for every %v hours", startTimeStr, configViper.GetString("ScheduleUpdateJobIntervalInHours")))

	s.StartBlocking()
}
func loadRepositoryOnStartup(airportCode string) {

	updateRepository(airportCode)

	// Schedule the automated scheduled pushes to for defined endpoints
	go schedulePushes(airportCode)

}

func updateRepository(airportCode string) {

	defer exeTime(fmt.Sprintf("Updated Repository for %s", airportCode))()
	// Update the resource map. New entries will be added, existing entries will be left untouched
	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Starting", airportCode))
	populateResourceMaps(airportCode)
	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Complete", airportCode))

	repo := GetRepo(airportCode)
	chunkSize := repo.LoadFlightChunkSizeInDays
	if chunkSize < 1 {
		chunkSize = 2
	}

	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Getting flights. Chunk Size: %v days", airportCode, chunkSize))

	for min := GetRepo(airportCode).FlightSDOWindowMinimumInDaysFromNow; min <= GetRepo(airportCode).FlightSDOWindowMaximumInDaysFromNow; min += chunkSize {
		var envel Envelope
		xml.Unmarshal(getFlights(airportCode, min, min+chunkSize), &envel)

		for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
			flight.LastUpdate = time.Now()
			flight.Action = StatusAction
			mapMutex.Lock()
			//(*repo).Flights = replaceOrAddFlight(repo.Flights, flight)
			repo.FlightLinkedList.ReplaceOrAddNode(flight)
			upadateAllocation(flight, airportCode)
			mapMutex.Unlock()
		}

		flightsInitChannel <- len(envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight)
	}

	from := time.Now().AddDate(0, 0, repo.FlightSDOWindowMinimumInDaysFromNow)
	to := time.Now().AddDate(0, 0, repo.FlightSDOWindowMaximumInDaysFromNow)

	fmt.Printf("Got flights set from %s to %s\n", from, to)

	(*repo).updateLowerLimit(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()))
	(*repo).updateUpperLimit(time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location()))

	//logger.Info(fmt.Sprintf("Repository updated for %s  Number of flights = %v", airportCode, len((*repo).Flights)))

	cleanRepository(from, airportCode)
}
func cleanRepository(from time.Time, airportCode string) {

	// Cleans the repository of old entries
	mapMutex.Lock()
	defer mapMutex.Unlock()

	logger.Info(fmt.Sprintf("Cleaning repository from: %s", from))
	GetRepo(airportCode).FlightLinkedList.RemoveExpiredNode(from)
}
func updateFlightEntry(message string) {

	var envel FlightUpdatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightUpdatedNotification.Flight

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)

	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, repo.FlightSDOWindowMinimumInDaysFromNow-2)) {
		logger.Debugf("Update for Flight Before Window. Flight ID: %s", flight.GetFlightID())
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, repo.FlightSDOWindowMaximumInDaysFromNow+2)) {
		logger.Debugf("Update for Flight After Window. Flight ID: %s", flight.GetFlightID())
		return
	}

	flight.LastUpdate = time.Now()
	flight.Action = UpdateAction

	mapMutex.Lock()
	repo.FlightLinkedList.ReplaceOrAddNode(flight)
	upadateAllocation(flight, airportCode)
	mapMutex.Unlock()

	flightUpdatedChannel <- flight
}
func createFlightEntry(message string) {

	var envel FlightCreatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightCreatedNotification.Flight
	flight.LastUpdate = time.Now()
	flight.Action = CreateAction

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)
	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, GetRepo(airportCode).FlightSDOWindowMinimumInDaysFromNow-2)) {
		log.Println("Create for Flight Before Window")
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, GetRepo(airportCode).FlightSDOWindowMaximumInDaysFromNow+2)) {
		log.Println("Create for Flight After Window")
		return
	}
	mapMutex.Lock()
	repo.FlightLinkedList.ReplaceOrAddNode(flight)
	upadateAllocation(flight, airportCode)
	mapMutex.Unlock()

	flightCreatedChannel <- flight
}
func deleteFlightEntry(message string) {

	//repo := repoMap[airportCode]

	var envel FlightDeletedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightDeletedNotification.Flight
	//flightID := flight.GetFlightID()
	flight.Action = DeleteAction

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)

	(*repo).FlightLinkedList.RemoveNode(flight)
	(*repo).RemoveFlightAllocation(flight.GetFlightID())

	flightDeletedChannel <- flight
}
func getFlights(airportCode string, values ...int) []byte {

	repo := GetRepo(airportCode)
	from := time.Now().AddDate(0, 0, repo.FlightSDOWindowMinimumInDaysFromNow).Format("2006-01-02")
	to := time.Now().AddDate(0, 0, repo.FlightSDOWindowMaximumInDaysFromNow+1).Format("2006-01-02")

	// Change the window based on optional inout parameters
	if len(values) >= 1 {
		from = time.Now().AddDate(0, 0, values[0]).Format("2006-01-02")
	}

	// Add in a sneaky extra day
	if len(values) >= 2 {
		to = time.Now().AddDate(0, 0, values[1]+1).Format("2006-01-02")
	}

	logger.Debug(fmt.Sprintf("Getting flight from %s to %s", from, to))
	fmt.Printf("Getting flights from %s to %s\n", from, to)

	queryBody := fmt.Sprintf(xmlBody, repo.AMSToken, from, to, repo.AMSAirport)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.AMSSOAPServiceURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not create request: %s\n", err))
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetFlights")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("client: error making http request: %s\n", err))
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not read response body: %s\n", err))
	}

	fmt.Printf("Got flights from %s to %s\n", from, to)
	return resBody
}
func upadateAllocation(flight Flight, airportCode string) {

	//defer exeTime(fmt.Sprintf("Updated allocations for Flight %s", flight.GetFlightID()))()
	// Testing with 3000 flights showed unmeasurable time to 500 micro seconds, so no worries mate

	repo := GetRepo(airportCode)
	// It's too messy to do CRUD operations, so just delete all the allocations and then create them again from the current message
	(*repo).RemoveFlightAllocation(flight.GetFlightID())

	flightId := flight.GetFlightID()
	direction := flight.GetFlightDirection()
	route := flight.GetFlightRoute()
	aircraftType := flight.GetAircraftType()
	aircraftRegistration := flight.GetAircraftRegistration()
	now := time.Now().Local()

	for _, checkInSlot := range flight.FlightState.CheckInSlots.CheckInSlot {
		checkInID, start, end := checkInSlot.getResourceID()

		allocation := AllocationItem{
			ResourceID:           checkInID,
			From:                 start,
			To:                   end,
			FlightID:             flightId,
			AirportCode:          airportCode,
			Direction:            direction,
			Route:                route,
			AircraftType:         aircraftType,
			AircraftRegistration: aircraftRegistration,
			LastUpdate:           now}

		(*repo).CheckInList.AddAllocation(allocation)

	}

	for _, gateSlot := range flight.FlightState.GateSlots.GateSlot {
		gateID, start, end := gateSlot.getResourceID()

		allocation := AllocationItem{
			ResourceID:           gateID,
			From:                 start,
			To:                   end,
			FlightID:             flightId,
			AirportCode:          airportCode,
			Direction:            direction,
			Route:                route,
			AircraftType:         aircraftType,
			AircraftRegistration: aircraftRegistration,
			LastUpdate:           now}

		(*repo).GateList.AddAllocation(allocation)
	}

	for _, standSlot := range flight.FlightState.StandSlots.StandSlot {
		standID, start, end := standSlot.getResourceID()

		allocation := AllocationItem{
			ResourceID:           standID,
			From:                 start,
			To:                   end,
			FlightID:             flightId,
			AirportCode:          airportCode,
			Direction:            direction,
			Route:                route,
			AircraftType:         aircraftType,
			AircraftRegistration: aircraftRegistration,
			LastUpdate:           now}

		(*repo).StandList.AddAllocation(allocation)
	}

	for _, carouselSlot := range flight.FlightState.CarouselSlots.CarouselSlot {
		carouselID, start, end := carouselSlot.getResourceID()

		allocation := AllocationItem{
			ResourceID:           carouselID,
			From:                 start,
			To:                   end,
			FlightID:             flightId,
			AirportCode:          airportCode,
			Direction:            direction,
			Route:                route,
			AircraftType:         aircraftType,
			AircraftRegistration: aircraftRegistration,
			LastUpdate:           now}

		(*repo).CarouselList.AddAllocation(allocation)

	}

	for _, chuteSlot := range flight.FlightState.ChuteSlots.ChuteSlot {
		chuteID, start, end := chuteSlot.getResourceID()

		allocation := AllocationItem{
			ResourceID:           chuteID,
			From:                 start,
			To:                   end,
			FlightID:             flightId,
			AirportCode:          airportCode,
			Direction:            direction,
			Route:                route,
			AircraftType:         aircraftType,
			AircraftRegistration: aircraftRegistration,
			LastUpdate:           now}

		(*repo).ChuteList.AddAllocation(allocation)
	}

	//fmt.Printf("FlightLinked List: %T, %d %d\n", repo.FlightLinkedList, repo.FlightLinkedList.Len(), unsafe.Sizeof(repo.FlightLinkedList))

}

// func deleteAllocation(flight Flight, airportCode string) {

// 	GetRepo(airportCode).RemoveFlightAllocation(flight.GetFlightID())
// 	flightId := flight.GetFlightID()

// 	(*repo).CheckInList.RemoveFlightAllocation(flightId)
// 	(*repo).GateList.RemoveFlightAllocation(flightId)
// 	(*repo).StandList.RemoveFlightAllocation(flightId)
// 	(*repo).CarouselList.RemoveFlightAllocation(flightId)
// 	(*repo).ChuteList.RemoveFlightAllocation(flightId)

// }

// Retrieve resources from AMS
func getResource(airportCode string, resourceType string) []byte {

	repo := GetRepo(airportCode)

	url := repo.AMSRestServiceURL + "/" + repo.AMSAirport + "/" + resourceType

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: Could not create request: %s\n", err))
		return nil
	}

	req.Header.Set("Authorization", repo.AMSToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: error making http request: %s\n", err))
		return nil
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: could not read response body: %s\n", err))
		return nil
	}

	return resBody
}

func testNativeAPIConnectivity(airportCode string) bool {

	repo := GetRepo(airportCode)

	queryBody := fmt.Sprintf(testNativeAPIMessage, repo.AMSToken)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.AMSSOAPServiceURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("Native API Test Client: could not create request: %s\n", err))
		return false
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetAirports")

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		logger.Error(fmt.Sprintf("Native API Test Client: error making http request: %s\n", err))
		return false
	}

	return true
}

func testRestAPIConnectivity(airportCode string) bool {
	repo := GetRepo(airportCode)

	url := repo.AMSRestServiceURL + "/" + repo.AMSAirport + "/Gates"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Test Connectivity Client: Could not create request: %s\n", err))
		return false
	}

	req.Header.Set("Authorization", repo.AMSToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		logger.Error(fmt.Sprintf("Test Connectivity Client: error making http request: %s\n", err))
		return false
	}

	_, err = io.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("Test Connectivity Client: could not read response body: %s\n", err))
		return false
	}

	return true
}
