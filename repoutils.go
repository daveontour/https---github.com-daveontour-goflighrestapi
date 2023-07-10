package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func InitRepositories() {

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, "airports.json"))

	if err != nil {
		log.Fatal(err)
		return
	}

	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var repos Repositories

	json.Unmarshal([]byte(byteResult), &repos)

	for _, v := range repos.Repositories {
		v.Flights = make(map[string]Flight)
		v.CarouselAllocationMap = make(map[string]ResourceAllocationMap)
		v.CheckInAllocationMap = make(map[string]ResourceAllocationMap)
		v.StandAllocationMap = make(map[string]ResourceAllocationMap)
		v.GateAllocationMap = make(map[string]ResourceAllocationMap)
		v.ChuteAllocationMap = make(map[string]ResourceAllocationMap)
		repoMap[v.Airport] = v
	}

	repositoryUpdateChannel <- 1

	for _, v := range repoMap {
		initRepository(&v)
	}
}

func addResourcesToMap(resources []FixedResource, mapp map[string]ResourceAllocationMap) map[string]ResourceAllocationMap {

	for _, c := range resources {
		r := ResourceAllocationMap{
			Resource:             c,
			FlightAllocationsMap: make(map[string]AllocationItem),
		}

		mapp[c.Name] = r
	}
	return mapp
}

func populateResourceMaps(repo *Repository, resourceWG *sync.WaitGroup) {

	var checkInAllocationMap = make(map[string]ResourceAllocationMap)
	var standAllocationMap = make(map[string]ResourceAllocationMap)
	var gateAllocationMap = make(map[string]ResourceAllocationMap)
	var carouselAllocationMap = make(map[string]ResourceAllocationMap)
	var chuteAllocationMap = make(map[string]ResourceAllocationMap)

	// Retrieve the available resources
	var checkIns FixedResources
	xml.Unmarshal(getResource(*repo, "CheckIns"), &checkIns)
	checkInAllocationMap = addResourcesToMap(checkIns.Values, checkInAllocationMap)

	var stands FixedResources
	xml.Unmarshal(getResource(*repo, "Stands"), &stands)
	standAllocationMap = addResourcesToMap(stands.Values, standAllocationMap)

	var gates FixedResources
	xml.Unmarshal(getResource(*repo, "Gates"), &gates)
	gateAllocationMap = addResourcesToMap(gates.Values, gateAllocationMap)

	var carousels FixedResources
	xml.Unmarshal(getResource(*repo, "Carousels"), &carousels)
	carouselAllocationMap = addResourcesToMap(carousels.Values, carouselAllocationMap)

	var chutes FixedResources
	xml.Unmarshal(getResource(*repo, "Chutes"), &chutes)
	chuteAllocationMap = addResourcesToMap(chutes.Values, chuteAllocationMap)

	repo.CheckInAllocationMap = checkInAllocationMap
	repo.StandAllocationMap = standAllocationMap
	repo.CarouselAllocationMap = carouselAllocationMap
	repo.GateAllocationMap = gateAllocationMap
	repo.ChuteAllocationMap = chuteAllocationMap

	repoMap[repo.Airport] = *repo

	resourceWG.Done()
}
func initRepository(repo *Repository) {

	// Break up the window into smaller chunks to avoid timeouts or single big hit
	//
	chunkSize := repo.ChunkSize
	if chunkSize < 1 {
		chunkSize = 2
	}

	var resourceWG sync.WaitGroup
	resourceWG.Add(1)
	go populateResourceMaps(repo, &resourceWG)

	// Retrieve the current flights
	repoMutex.Lock()
	for min := repo.WindowMin; min <= repo.WindowMax; min += chunkSize {
		var envel Envelope
		xml.Unmarshal(getFlights(*repo, min, min+chunkSize), &envel)

		//Wait until the Resource have all been gotten
		resourceWG.Wait()

		//		if entry, ok := repoMap[repo.Airport]; ok {
		for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
			flight.LastUpdate = time.Now()
			repo.Flights[flight.GetFlightID()] = flight
			repo.FlightList.insert(flight)
			upadateAllocation(flight, repo)
		}
		//			repoMap[repo.Airport] = entry
		//		}
	}

	fmt.Println("Repository loaded for ", repo.Airport, "  Number of flights = ", len(repoMap[repo.Airport].Flights))

	entry, _ := repoMap[repo.Airport]

	from := time.Now().AddDate(0, 0, repo.WindowMin)
	to := time.Now().AddDate(0, 0, repo.WindowMax)

	entry.CurrentLowerLimit = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	entry.CurrentUpperLimit = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

	repoMap[repo.Airport] = entry

	repoMutex.Unlock()

	go maintainRepository(*repo)

}

func maintainRepository(repo Repository) {

	// Schedule the regular refresh
	go scheduleUpdates(repo)

	//Listen to the notification queue
	opts := []msmq.QueueInfoOption{
		msmq.WithPathName(repo.ListenerQueue),
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

		//fileContent, err := os.Open("c:\\Users\\dave_\\Desktop\\test.xml")

		// if err != nil {
		// 	log.Fatal(err)
		// 	return
		// }

		//defer fileContent.Close()

		//byteResult, _ := ioutil.ReadAll(fileContent)

		message, err := msg.Body()

		//message := string(byteResult)
		if strings.Contains(message, "FlightUpdatedNotification") {
			updateFlightEntry(message, repo)
			continue
		}
		if strings.Contains(message, "FlightCreatedNotification") {
			createFlightEntry(message, repo)
			continue
		}
		if strings.Contains(message, "FlightDeletedNotification") {
			deleteFlightEntry(message, repo)
			continue
		}
		fmt.Println("Unhandled Message Type Received")
	}
}

func scheduleUpdates(repo Repository) {
	// Schedule the regular refresh
	s := gocron.NewScheduler(time.UTC)
	s.Every(24).Hours().Do(func() { updateRepository(repo) })
	fmt.Println("Scheduled")
	s.StartBlocking()
}

func updateRepository(repo Repository) {

	fmt.Println("Maintinaing Repository:", repo.Airport)
	var envel Envelope
	xml.Unmarshal(getFlights(repo), &envel)

	repoMutex.Lock()
	defer repoMutex.Unlock()

	if entry, ok := repoMap[repo.Airport]; ok {
		for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
			flight.LastUpdate = time.Now()
			entry.Flights[flight.GetFlightID()] = flight
		}

		from := time.Now().AddDate(0, 0, repo.WindowMin)
		to := time.Now().AddDate(0, 0, repo.WindowMax)

		entry.CurrentLowerLimit = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
		entry.CurrentUpperLimit = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

		repoMap[repo.Airport] = entry

		fmt.Println("Repository updated for ", repo.Airport, "  Number of flights = ", len(entry.Flights))

	}
}

func updateFlightEntry(message string, repo Repository) {

	var envel FlightUpdatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightUpdatedNotification.Flight
	flight.LastUpdate = time.Now()

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
	if airportentry, ok := repoMap[repo.Airport]; ok {
		airportentry.Flights[flightID] = flight
	}
	repoMutex.Unlock()

	upadateAllocation(flight, &repo)

	flightUpdatedChannel <- envel.Content.FlightUpdatedNotification.Flight

}
func createFlightEntry(message string, repo Repository) {
	var envel FlightCreatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightCreatedNotification.Flight
	flight.LastUpdate = time.Now()

	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, repo.WindowMin-2)) {
		log.Println("Create for Flight Before Window")
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, repo.WindowMax+2)) {
		log.Println("Create for Flight After Window")
		return
	}
	repoMutex.Lock()
	repoMap[repo.Airport].Flights[flight.GetFlightID()] = flight
	repoMutex.Unlock()

	upadateAllocation(flight, &repo)
	flightCreatedChannel <- envel.Content.FlightCreatedNotification.Flight
}
func deleteFlightEntry(message string, repo Repository) {
	var envel FlightDeletedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightDeletedNotification.Flight
	flightID := flight.GetFlightID()

	repoMutex.Lock()
	if airportentry, ok := repoMap[repo.Airport]; ok {
		delete(airportentry.Flights, flightID)
	}
	repoMutex.Unlock()

	deleteAllocation(flight, &repo)
	flightDeletedChannel <- envel.Content.FlightDeletedNotification.Flight
}

func updateRepositoryWindow(repo Repository) {

}

func getFlights(repo Repository, values ...int) []byte {

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

	fmt.Println("Getting flight from", from, "to", to)

	queryBody := fmt.Sprintf(xmlBody, repo.Token, from, to, repo.Airport)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.URL, bodyReader)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetFlights")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		os.Exit(1)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("client: could not read response body: %s\n", err)
		os.Exit(1)
	}

	return resBody
}

func getResource(repo Repository, resourceType string) []byte {

	url := repo.RestURL + "/" + repo.Airport + "/" + resourceType

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", repo.Token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		os.Exit(1)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("client: could not read response body: %s\n", err)
		os.Exit(1)
	}

	return resBody
}
