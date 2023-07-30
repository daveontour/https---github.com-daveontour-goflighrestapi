package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
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
	for _, repo := range repoList {
		if repo.Airport == airportCode {
			return &repo
		}
	}
	return nil
}

func InitRepositories() {

	var repos Repositories
	json.Unmarshal([]byte(readBytesFromFile("airports.json")), &repos)

	for _, v := range repos.Repositories {
		v.Flights = make(map[string]Flight)
		v.CarouselAllocationMap = make(map[string]ResourceAllocationMap)
		v.CheckInAllocationMap = make(map[string]ResourceAllocationMap)
		v.StandAllocationMap = make(map[string]ResourceAllocationMap)
		v.GateAllocationMap = make(map[string]ResourceAllocationMap)
		v.ChuteAllocationMap = make(map[string]ResourceAllocationMap)
		repoList = append(repoList, v)
	}

	for _, v := range repoList {
		go initRepository(v.Airport)
	}
}

func reInitAirport(aptCode string) {

	var repos Repositories
	json.Unmarshal([]byte(readBytesFromFile("airports.json")), &repos)

	for _, v := range repos.Repositories {
		if v.Airport != aptCode {
			continue
		}
		v.Flights = make(map[string]Flight)
		v.CarouselAllocationMap = make(map[string]ResourceAllocationMap)
		v.CheckInAllocationMap = make(map[string]ResourceAllocationMap)
		v.StandAllocationMap = make(map[string]ResourceAllocationMap)
		v.GateAllocationMap = make(map[string]ResourceAllocationMap)
		v.ChuteAllocationMap = make(map[string]ResourceAllocationMap)
		repoList = append(repoList, v)
	}

	s := refreshSchedulerMap[aptCode]
	if s != nil {
		s.Clear()
	}

	go initRepository(aptCode)

}

func initRepository(airportCode string) {

	//Make sure the required services are available
	for !testNativeAPI(airportCode) || !testRestAPIConnectivity(airportCode) {
		logger.Warn(fmt.Sprintf("AMS Webservice API or AMS RestAPI not avaiable for %s. Will try again in 8 seconds", airportCode))
		time.Sleep(8 * time.Second)
	}

	// Purge the listening queue first before doing the Initializarion of the repository
	opts := []msmq.QueueInfoOption{
		msmq.WithPathName(GetRepo(airportCode).ListenerQueue),
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

	logger.Info(fmt.Sprintf("Populating Resource Maps for %s", airportCode))
	// Retrieve the available resources

	var checkIns FixedResources
	xml.Unmarshal(getResource(airportCode, "CheckIns"), &checkIns)
	addResourcesToMap(checkIns.Values, GetRepo(airportCode).CheckInAllocationMap)

	var stands FixedResources
	xml.Unmarshal(getResource(airportCode, "Stands"), &stands)
	addResourcesToMap(stands.Values, GetRepo(airportCode).StandAllocationMap)

	var gates FixedResources
	xml.Unmarshal(getResource(airportCode, "Gates"), &gates)
	addResourcesToMap(gates.Values, GetRepo(airportCode).GateAllocationMap)

	var carousels FixedResources
	xml.Unmarshal(getResource(airportCode, "Carousels"), &carousels)
	addResourcesToMap(carousels.Values, GetRepo(airportCode).CarouselAllocationMap)

	var chutes FixedResources
	xml.Unmarshal(getResource(airportCode, "Chutes"), &chutes)
	addResourcesToMap(chutes.Values, GetRepo(airportCode).ChuteAllocationMap)

	logger.Info(fmt.Sprintf("Completed Populating Resource Maps for %s", airportCode))
}

func addResourcesToMap(resources []FixedResource, mapp map[string]ResourceAllocationMap) map[string]ResourceAllocationMap {

	mapMutex.Lock()
	for _, c := range resources {
		r := ResourceAllocationMap{
			Resource:             c,
			FlightAllocationsMap: make(map[string]AllocationItem),
		}

		//Only add it to the map if it doesn't already exist
		if _, ok := mapp[c.Name]; !ok {
			mapp[c.Name] = r
		}

	}
	mapMutex.Unlock()

	return mapp
}

func maintainRepository(airportCode string) {

	// Schedule the regular refresh
	go scheduleUpdates(airportCode)

	//Listen to the notification queue
	opts := []msmq.QueueInfoOption{
		msmq.WithPathName(GetRepo(airportCode).ListenerQueue),
	}
	queueInfo, err := msmq.NewQueueInfo(opts...)
	if err != nil {
		log.Fatal(err)
	}

	for {
		queue, err := queueInfo.Open(msmq.Receive, msmq.DenyNone)

		if err != nil {
			log.Fatal(err)
			continue
		}

		msg, err := queue.Receive() //This call blocks until a message is available
		if err != nil {
			log.Fatal(err)
			continue
		}

		message, err := msg.Body()

		if strings.Contains(message, "FlightUpdatedNotification") {
			updateFlightEntry(message)
			continue
		}
		if strings.Contains(message, "FlightCreatedNotification") {
			createFlightEntry(message)
			continue
		}
		if strings.Contains(message, "FlightDeletedNotification") {
			deleteFlightEntry(message)
			continue
		}
	}
}
func scheduleUpdates(airportCode string) {

	// Schedule the regular refresh

	today := time.Now().Format("2006-01-02")
	startTimeStr := today + "T" + serviceConfig.ScheduleUpdateJob
	startTime, _ := time.ParseInLocation("2006-01-02T15:04:05", startTimeStr, loc)

	s := gocron.NewScheduler(time.Local)

	refreshSchedulerMap[airportCode] = s

	// Schedule the regular update of the repositoiry
	s.Every(serviceConfig.ScheduleUpdateJobIntervalInHours).Hours().StartAt(startTime).Do(func() { updateRepository(airportCode) })

	// Kick off an intial load on startup
	s.Every(1).Millisecond().LimitRunsTo(1).Do(func() { loadRepositoryOnStartup(airportCode) })

	logger.Info(fmt.Sprintf("Regular updates of the repository have been scheduled at %s for every %v hours", startTimeStr, serviceConfig.ScheduleUpdateJobIntervalInHours))

	s.StartBlocking()
}
func loadRepositoryOnStartup(airportCode string) {

	updateRepository(airportCode)

	// Schedule the automated scheduled pushes to for defined endpoints
	go schedulePushes(airportCode)

}

func updateRepository(airportCode string) {

	// Update the resource map. New entries will be added, existing entries will be left untouched
	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Starting", airportCode))
	populateResourceMaps(airportCode)
	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Complete", airportCode))

	repo := GetRepo(airportCode)
	chunkSize := repo.ChunkSize
	if chunkSize < 1 {
		chunkSize = 2
	}

	logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Getting flights. Chunk Size: %v days", airportCode, chunkSize))

	repoMutex.Lock()
	defer repoMutex.Unlock()

	for min := GetRepo(airportCode).WindowMin; min <= GetRepo(airportCode).WindowMax; min += chunkSize {
		var envel Envelope
		xml.Unmarshal(getFlights(airportCode, min, min+chunkSize), &envel)

		for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
			flight.LastUpdate = time.Now()
			flight.Action = StatusAction
			GetRepo(airportCode).Flights[flight.GetFlightID()] = flight
			upadateAllocation(flight, airportCode)
		}

		flightsInitChannel <- len(envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight)
	}

	from := time.Now().AddDate(0, 0, GetRepo(airportCode).WindowMin)
	to := time.Now().AddDate(0, 0, GetRepo(airportCode).WindowMax)

	GetRepo(airportCode).updateLowerLimit(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()))
	GetRepo(airportCode).updateUpperLimit(time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location()))

	logger.Info(fmt.Sprintf("Repository updated for %s  Number of flights = %v", airportCode, len(GetRepo(airportCode).Flights)))

	cleanRepository(from, airportCode)
}
func cleanRepository(from time.Time, airportCode string) {

	// Cleans the repository of old entries

	logger.Info(fmt.Sprintf("Cleaning repository from: %s", from))
	flights := GetRepo(airportCode).Flights
	remove := []Flight{}

	for _, f := range flights {
		if f.GetSTO().Before(from) {
			deleteAllocation(f, airportCode)
			remove = append(remove, f)
		}
	}

	for _, f := range remove {
		delete(GetRepo(airportCode).Flights, f.GetFlightID())
	}
	logger.Info(fmt.Sprintf("Repository Cleaned for %s  Number of flights = %v", airportCode, len(remove)))
}
func updateFlightEntry(message string) {

	var envel FlightUpdatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightUpdatedNotification.Flight
	flight.LastUpdate = time.Now()
	flight.Action = UpdateAction

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)

	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, repo.WindowMin-2)) {
		log.Println("Update for Flight Before Window")
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, repo.WindowMax+2)) {
		log.Println("Update for Flight After Window")
		return
	}

	flightID := flight.GetFlightID()

	repoMutex.Lock()

	GetRepo(airportCode).Flights[flightID] = flight
	repoMutex.Unlock()

	upadateAllocation(flight, airportCode)

	flightUpdatedChannel <- flight
}
func createFlightEntry(message string) {

	var envel FlightCreatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightCreatedNotification.Flight
	flight.LastUpdate = time.Now()
	flight.Action = CreateAction

	airportCode := flight.GetIATAAirport()
	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, GetRepo(airportCode).WindowMin-2)) {
		log.Println("Create for Flight Before Window")
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, GetRepo(airportCode).WindowMax+2)) {
		log.Println("Create for Flight After Window")
		return
	}
	repoMutex.Lock()
	GetRepo(airportCode).Flights[flight.GetFlightID()] = flight
	repoMutex.Unlock()

	upadateAllocation(flight, airportCode)
	flightCreatedChannel <- flight
}
func deleteFlightEntry(message string) {

	//repo := repoMap[airportCode]

	var envel FlightDeletedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightDeletedNotification.Flight
	flightID := flight.GetFlightID()
	flight.Action = DeleteAction

	airportCode := flight.GetIATAAirport()

	repoMutex.Lock()
	//if airportentry, ok := repoMap[repo.Airport]; ok {
	delete(GetRepo(airportCode).Flights, flightID)
	//}
	repoMutex.Unlock()

	deleteAllocation(flight, airportCode)
	flightDeletedChannel <- flight
}
func getFlights(airportCode string, values ...int) []byte {

	repo := GetRepo(airportCode)
	from := time.Now().AddDate(0, 0, repo.WindowMin).Format("2006-01-02")
	to := time.Now().AddDate(0, 0, repo.WindowMax+1).Format("2006-01-02")

	// Change the window based on optional inout parameters
	if len(values) >= 1 {
		from = time.Now().AddDate(0, 0, values[0]).Format("2006-01-02")
	}

	// Add in a sneaky extra day
	if len(values) >= 2 {
		to = time.Now().AddDate(0, 0, values[1]+1).Format("2006-01-02")
	}

	logger.Debug(fmt.Sprintf("Getting flight from %s to %s", from, to))

	queryBody := fmt.Sprintf(xmlBody, repo.Token, from, to, repo.Airport)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.URL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not create request: %s\n", err))
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetFlights")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("client: error making http request: %s\n", err))
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not read response body: %s\n", err))
	}

	return resBody
}
func upadateAllocation(flight Flight, airportCode string) {

	repo := GetRepo(airportCode)
	// It's too messy to do CRUD operations, so just delete all the allocations and then create them again from the current message

	deleteAllocation(flight, airportCode)

	flightId := flight.GetFlightID()
	direction := flight.GetFlightDirection()
	route := flight.GetFlightRoute()
	aircraftType := flight.GetAircraftType()
	aircraftRegistration := flight.GetAircraftRegistration()
	now := time.Now().Local()

	for _, checkInSlot := range flight.FlightState.CheckInSlots.CheckInSlot {
		checkInID, start, end := checkInSlot.getResourceID()

		_, ok := repo.CheckInAllocationMap[checkInID]
		if ok {
			repo.CheckInAllocationMap[checkInID].FlightAllocationsMap[flightId] = AllocationItem{
				From:                 start,
				To:                   end,
				FlightID:             flightId,
				AirportCode:          airportCode,
				Direction:            direction,
				Route:                route,
				AircraftType:         aircraftType,
				AircraftRegistration: aircraftRegistration,
				LastUpdate:           now}
		}
	}

	for _, gateSlot := range flight.FlightState.GateSlots.GateSlot {
		gateID, start, end := gateSlot.getResourceID()
		_, ok := repo.GateAllocationMap[gateID]
		if ok {
			repo.GateAllocationMap[gateID].FlightAllocationsMap[flightId] = AllocationItem{
				From:                 start,
				To:                   end,
				FlightID:             flightId,
				AirportCode:          airportCode,
				Direction:            direction,
				Route:                route,
				AircraftType:         aircraftType,
				AircraftRegistration: aircraftRegistration,
				LastUpdate:           now}
		}
	}

	for _, standSlot := range flight.FlightState.StandSlots.StandSlot {
		standID, start, end := standSlot.getResourceID()
		_, ok := repo.StandAllocationMap[standID]
		if ok {
			repo.StandAllocationMap[standID].FlightAllocationsMap[flightId] = AllocationItem{
				From:                 start,
				To:                   end,
				FlightID:             flightId,
				AirportCode:          airportCode,
				Direction:            direction,
				Route:                route,
				AircraftType:         aircraftType,
				AircraftRegistration: aircraftRegistration,
				LastUpdate:           now}
		}
	}

	for _, carouselSlot := range flight.FlightState.CarouselSlots.CarouselSlot {
		carouselID, start, end := carouselSlot.getResourceID()
		_, ok := repo.CarouselAllocationMap[carouselID]
		if ok {
			repo.CarouselAllocationMap[carouselID].FlightAllocationsMap[flightId] = AllocationItem{
				From:                 start,
				To:                   end,
				FlightID:             flightId,
				AirportCode:          airportCode,
				Direction:            direction,
				Route:                route,
				AircraftType:         aircraftType,
				AircraftRegistration: aircraftRegistration,
				LastUpdate:           now}
		}
	}

	for _, chuteSlot := range flight.FlightState.ChuteSlots.ChuteSlot {
		chuteID, start, end := chuteSlot.getResourceID()
		_, ok := repo.ChuteAllocationMap[chuteID]
		if ok {
			repo.ChuteAllocationMap[chuteID].FlightAllocationsMap[flightId] = AllocationItem{
				From:                 start,
				To:                   end,
				FlightID:             flightId,
				AirportCode:          airportCode,
				Direction:            direction,
				Route:                route,
				AircraftType:         aircraftType,
				AircraftRegistration: aircraftRegistration,
				LastUpdate:           now}
		}
	}
}

func deleteAllocation(flight Flight, airportCode string) {

	repo := GetRepo(airportCode)
	flightId := flight.GetFlightID()

	mapMutex.Lock()

	for _, v := range repo.CheckInAllocationMap {
		delete(v.FlightAllocationsMap, flightId)
	}
	for _, v := range repo.GateAllocationMap {
		delete(v.FlightAllocationsMap, flightId)
	}
	for _, v := range repo.StandAllocationMap {
		delete(v.FlightAllocationsMap, flightId)
	}
	for _, v := range repo.CarouselAllocationMap {
		delete(v.FlightAllocationsMap, flightId)
	}
	for _, v := range repo.ChuteAllocationMap {
		delete(v.FlightAllocationsMap, flightId)
	}

	mapMutex.Unlock()
}

// Retrieve resources from AMS
func getResource(airportCode string, resourceType string) []byte {

	repo := GetRepo(airportCode)

	url := repo.RestURL + "/" + repo.Airport + "/" + resourceType

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: Could not create request: %s\n", err))
		return nil
	}

	req.Header.Set("Authorization", repo.Token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: error making http request: %s\n", err))
		return nil
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("Get Resource Client: could not read response body: %s\n", err))
		return nil
	}

	return resBody
}

func testNativeAPI(airportCode string) bool {

	repo := GetRepo(airportCode)

	queryBody := fmt.Sprintf(testNativeAPIMessage, repo.Token)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.URL, bodyReader)
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

	url := repo.RestURL + "/" + repo.Airport + "/Gates"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Test Connectivity Client: Could not create request: %s\n", err))
		return false
	}

	req.Header.Set("Authorization", repo.Token)

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		logger.Error(fmt.Sprintf("Test Connectivity Client: error making http request: %s\n", err))
		return false
	}

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("Test Connectivity Client: could not read response body: %s\n", err))
		return false
	}

	return true
}
