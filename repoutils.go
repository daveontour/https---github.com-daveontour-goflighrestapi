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

func InitRepositories() {
	fileContent, err := os.Open("airports.json")

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
		repoMap[v.Airport] = v
	}

	repositoryUpdateChannel <- 1

	for _, v := range repos.Repositories {
		initRepository(v)
	}
}

func initRepository(repo Repository) {

	// Break up the window into smaller chunks to avoid timeouts or single big hit
	//
	chunkSize := repo.ChunkSize
	if chunkSize < 1 {
		chunkSize = 2
	}

	repoMutex.Lock()
	for min := repo.WindowMin; min <= repo.WindowMax; min += chunkSize {
		var envel Envelope
		xml.Unmarshal(getFlights(repo, min, min+chunkSize), &envel)

		if entry, ok := repoMap[repo.Airport]; ok {
			for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
				entry.Flights[flight.GetFlightID()] = flight
			}
			repoMap[repo.Airport] = entry

		}
	}

	fmt.Println("Repository loaded for ", repo.Airport, "  Number of flights = ", len(repoMap[repo.Airport].Flights))

	entry, _ := repoMap[repo.Airport]

	from := time.Now().AddDate(0, 0, repo.WindowMin)
	to := time.Now().AddDate(0, 0, repo.WindowMax)

	entry.CurrentLowerLimit = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	entry.CurrentUpperLimit = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

	repoMap[repo.Airport] = entry

	repoMutex.Unlock()
	go maintainRepository(repo)

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

	flightUpdatedChannel <- envel.Content.FlightUpdatedNotification.Flight

}
func createFlightEntry(message string, repo Repository) {
	var envel FlightCreatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightCreatedNotification.Flight

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
