package repo

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

	"flightresourcerestapi/globals"
	"flightresourcerestapi/models"
	"flightresourcerestapi/timeservice"

	amqp "github.com/rabbitmq/amqp091-go"
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

func GetRepo(airportCode string) *models.Repository {
	for idx, repo := range globals.RepoList {
		if repo.AMSAirport == airportCode {
			return &globals.RepoList[idx]
		}
	}
	return nil
}

func InitRepositories() {

	var repos models.Repositories

	err := globals.AirportsViper.Unmarshal(&repos)
	if err != nil {
		fmt.Println(err)
	}

	if !globals.ConfigViper.GetBool("PerfTestOnly") {
		for _, v := range repos.Repositories {
			globals.RepoList = append(globals.RepoList, v)
			go initRepository(v.AMSAirport)
		}
	}
}

func ReInitAirport(aptCode string) {

	var repos models.Repositories
	globals.AirportsViper.ReadInConfig()
	globals.AirportsViper.Unmarshal(&repos)

	for _, v := range repos.Repositories {
		if v.AMSAirport != aptCode {
			continue
		}
		globals.RepoList = append(globals.RepoList, v)
	}

	s := globals.RefreshSchedulerMap[aptCode]
	if s != nil {
		s.Clear()
	}

	go initRepository(aptCode)

}

func initRepository(airportCode string) {

	defer globals.ExeTime(fmt.Sprintf("Initialising Repository for %s", airportCode))()

	if globals.ConfigViper.GetBool("PerfTestOnly") {
		//go test.SendUpdateMessages(5000)

	} else {
		//Make sure the required services are available
		for !testNativeAPIConnectivity(airportCode) || !testRestAPIConnectivity(airportCode) {
			globals.Logger.Warn(fmt.Sprintf("AMS Webservice API or AMS RestAPI not avaiable for %s. Will try again in 8 seconds", airportCode))
			time.Sleep(8 * time.Second)
		}

		repo := GetRepo(airportCode)

		if repo.ListenerType == "MSMQ" {
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
					if globals.IsDebug {
						globals.Logger.Error("Error purging listening queue")
					}
				} else {
					if globals.IsDebug {
						globals.Logger.Info("Listening queue purged OK")
					}
				}
			}
		}

		// The Maintence job schedules a repository population which inits the system
		populateResourceMaps(airportCode)
		go MaintainRepository(airportCode)
	}
}

func populateResourceMaps(airportCode string) {

	repo := GetRepo(airportCode)
	globals.Logger.Info(fmt.Sprintf("Populating Resource Maps for %s", airportCode))
	// Retrieve the available resources

	var checkIns models.FixedResources
	xml.Unmarshal(getResource(airportCode, "CheckIns"), &checkIns)
	(*repo).CheckInList.AddNodes(checkIns.Values)

	var stands models.FixedResources
	xml.Unmarshal(getResource(airportCode, "Stands"), &stands)
	(*repo).StandList.AddNodes(stands.Values)

	var gates models.FixedResources
	xml.Unmarshal(getResource(airportCode, "Gates"), &gates)
	(*repo).GateList.AddNodes(gates.Values)

	var carousels models.FixedResources
	xml.Unmarshal(getResource(airportCode, "Carousels"), &carousels)
	(*repo).CarouselList.AddNodes(carousels.Values)

	var chutes models.FixedResources
	xml.Unmarshal(getResource(airportCode, "Chutes"), &chutes)
	(*repo).ChuteList.AddNodes(chutes.Values)

	globals.Logger.Info(fmt.Sprintf("Completed Populating Resource Maps for %s", airportCode))
}

func MaintainRepository(airportCode string) {

	// Schedule the regular refresh
	go scheduleUpdates(airportCode)

	repo := GetRepo(airportCode)

	if repo.ListenerType == "MSMQ" {
		//Listen to the notification queue
		opts := []msmq.QueueInfoOption{
			msmq.WithPathName(GetRepo(airportCode).NotificationListenerQueue),
		}
		queueInfo, err := msmq.NewQueueInfo(opts...)
		if err != nil {
			log.Fatal(err)
		}

	ReconnectMSMQ:
		for {

			queue, err := queueInfo.Open(msmq.Receive, msmq.DenyNone)
			if err != nil {
				globals.Logger.Error(err)
				continue ReconnectMSMQ
			}

			for {

				msg, err := queue.Receive() //This call blocks until a message is available
				if err != nil {
					globals.Logger.Error(err)
					continue ReconnectMSMQ
				}

				message, _ := msg.Body()

				globals.Logger.Debug(fmt.Sprintf("Received Message length %d\n", len(message)))

				if strings.Contains(message, "FlightUpdatedNotification") {
					go UpdateFlightEntry(message)
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
	} else if repo.ListenerType == "RMQ" {
		conn, err := amqp.Dial(repo.RabbitMQConnectionString)
		failOnError(err, "Failed to connect to RabbitMQ")
		defer conn.Close()

		ch, err := conn.Channel()
		failOnError(err, "Failed to open a channel")
		defer ch.Close()

		//the session queue that will receive the messages from the Topic publisher
		q, err := ch.QueueDeclare(
			"",    // name
			false, // durable
			false, // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		failOnError(err, "Failed to declare a queue")

		log.Printf("Binding queue %s to exchange %s with routing key %s", q.Name, repo.RabbitMQExchange, repo.RabbitMQTopic)

		// Bind the seession queue to the Publisher
		err = ch.QueueBind(
			q.Name,                // queue name
			repo.RabbitMQTopic,    // routing key
			repo.RabbitMQExchange, // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")

		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto ack
			false,  // exclusive
			false,  // no local
			false,  // no wait
			nil,    // args
		)
		failOnError(err, "Failed to register a consumer")

		var forever chan struct{}

		// Read the messages from the queue
		go func() {
			for d := range msgs {
				globals.Logger.Debug("Rabbit Message Received")
				message := string(d.Body[:])

				globals.Logger.Debug(fmt.Sprintf("Received Message length %d\n", len(message)))

				if strings.Contains(message, "FlightUpdatedNotification") {
					go UpdateFlightEntry(message)
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
		}()

		log.Printf(" [*] Waiting for logs. To exit press CTRL+C")
		<-forever
	}
}
func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
func scheduleUpdates(airportCode string) {

	// Schedule the regular refresh

	today := time.Now().Format("2006-01-02")
	startTimeStr := today + "T" + globals.ConfigViper.GetString("ScheduleUpdateJob")
	startTime, _ := time.ParseInLocation("2006-01-02T15:04:05", startTimeStr, timeservice.Loc)

	s := gocron.NewScheduler(time.Local)

	globals.RefreshSchedulerMap[airportCode] = s

	// Schedule the regular update of the repositoiry
	s.Every(globals.ConfigViper.GetString("ScheduleUpdateJobIntervalInHours")).Hours().StartAt(startTime).Do(func() { updateRepository(airportCode) })

	// Kick off an intial load on startup
	s.Every(1).Millisecond().LimitRunsTo(1).Do(func() { loadRepositoryOnStartup(airportCode) })

	globals.Logger.Info(fmt.Sprintf("Regular updates of the repository have been scheduled at %s for every %v hours", startTimeStr, globals.ConfigViper.GetString("ScheduleUpdateJobIntervalInHours")))

	s.StartBlocking()
}
func loadRepositoryOnStartup(airportCode string) {

	updateRepository(airportCode)

	// Schedule the automated scheduled pushes to for defined endpoints
	go SchedulePushes(airportCode)

}

func updateRepository(airportCode string) {

	defer globals.ExeTime(fmt.Sprintf("Updated Repository for %s", airportCode))()
	// Update the resource map. New entries will be added, existing entries will be left untouched
	globals.Logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Starting", airportCode))
	populateResourceMaps(airportCode)
	globals.Logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Updating Resource Map - Complete", airportCode))

	repo := GetRepo(airportCode)
	chunkSize := repo.LoadFlightChunkSizeInDays
	if chunkSize < 1 {
		chunkSize = 2
	}

	globals.Logger.Info(fmt.Sprintf("Scheduled Maintenance of Repository: %s. Getting flights. Chunk Size: %v days", airportCode, chunkSize))

	for min := GetRepo(airportCode).FlightSDOWindowMinimumInDaysFromNow; min <= GetRepo(airportCode).FlightSDOWindowMaximumInDaysFromNow; min += chunkSize {
		var envel models.Envelope
		xml.Unmarshal(getFlights(airportCode, min, min+chunkSize), &envel)

		for _, flight := range envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight {
			flight.LastUpdate = time.Now()
			flight.Action = globals.StatusAction
			//	globals.MapMutex.Lock()
			(*repo).FlightLinkedList.ReplaceOrAddNode(flight)
			upadateAllocation(flight, airportCode)
			//	globals.MapMutex.Unlock()
		}

		globals.FlightsInitChannel <- len(envel.Body.GetFlightsResponse.GetFlightsResult.WebServiceResult.ApiResponse.Data.Flights.Flight)
	}

	from := time.Now().AddDate(0, 0, repo.FlightSDOWindowMinimumInDaysFromNow)
	to := time.Now().AddDate(0, 0, repo.FlightSDOWindowMaximumInDaysFromNow)

	fmt.Printf("Got flights set from %s to %s\n", from, to)

	(*repo).UpdateLowerLimit(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()))
	(*repo).UpdateUpperLimit(time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location()))

	cleanRepository(from, airportCode)
}
func cleanRepository(from time.Time, airportCode string) {

	// Cleans the repository of old entries
	// globals.MapMutex.Lock()
	// defer globals.MapMutex.Unlock()

	globals.Logger.Info(fmt.Sprintf("Cleaning repository from: %s", from))
	GetRepo(airportCode).FlightLinkedList.RemoveExpiredNode(from)
}

func UpdateFlightEntry(message string) {

	var envel models.FlightUpdatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightUpdatedNotification.Flight

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)

	if repo == nil {
		globals.Logger.Warn(fmt.Sprintf("Message for unmanaged airport %s received", airportCode))
		return
	}

	sdot := flight.GetSDO()

	if sdot.Before(time.Now().AddDate(0, 0, repo.FlightSDOWindowMinimumInDaysFromNow-2)) {
		globals.Logger.Debugf("Update for Flight Before Window. Flight ID: %s", flight.GetFlightID())
		return
	}
	if sdot.After(time.Now().AddDate(0, 0, repo.FlightSDOWindowMaximumInDaysFromNow+2)) {
		globals.Logger.Debugf("Update for Flight After Window. Flight ID: %s", flight.GetFlightID())
		return
	}

	flight.LastUpdate = time.Now()
	flight.Action = globals.UpdateAction

	globals.MapMutex.Lock()
	repo.FlightLinkedList.ReplaceOrAddNode(flight)
	upadateAllocation(flight, airportCode)
	globals.MapMutex.Unlock()

	globals.FlightUpdatedChannel <- flight
}
func createFlightEntry(message string) {

	var envel models.FlightCreatedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightCreatedNotification.Flight
	flight.LastUpdate = time.Now()
	flight.Action = globals.CreateAction

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
	//globals.MapMutex.Lock()
	repo.FlightLinkedList.ReplaceOrAddNode(flight)
	upadateAllocation(flight, airportCode)
	//globals.MapMutex.Unlock()

	globals.FlightCreatedChannel <- flight
}
func deleteFlightEntry(message string) {

	var envel models.FlightDeletedNotificatioEnvelope
	xml.Unmarshal([]byte(message), &envel)

	flight := envel.Content.FlightDeletedNotification.Flight
	flight.Action = globals.DeleteAction

	airportCode := flight.GetIATAAirport()
	repo := GetRepo(airportCode)

	(*repo).FlightLinkedList.RemoveNode(flight)
	(*repo).RemoveFlightAllocation(flight.GetFlightID())

	globals.FlightDeletedChannel <- flight
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

	globals.Logger.Debug(fmt.Sprintf("Getting flight from %s to %s", from, to))
	fmt.Printf("Getting flights from %s to %s\n", from, to)

	queryBody := fmt.Sprintf(xmlBody, repo.AMSToken, from, to, repo.AMSAirport)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, repo.AMSSOAPServiceURL, bodyReader)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("client: could not create request: %s\n", err))
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetFlights")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("client: error making http request: %s\n", err))
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("client: could not read response body: %s\n", err))
	}

	fmt.Printf("Got flights from %s to %s\n", from, to)
	return resBody
}
func upadateAllocation(flight models.Flight, airportCode string) {

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
		checkInID, start, end := checkInSlot.GetResourceID()

		allocation := models.AllocationItem{
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
		gateID, start, end := gateSlot.GetResourceID()

		allocation := models.AllocationItem{
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
		standID, start, end := standSlot.GetResourceID()

		allocation := models.AllocationItem{
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
		carouselID, start, end := carouselSlot.GetResourceID()

		allocation := models.AllocationItem{
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
		chuteID, start, end := chuteSlot.GetResourceID()

		allocation := models.AllocationItem{
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
}

// Retrieve resources from AMS
func getResource(airportCode string, resourceType string) []byte {

	repo := GetRepo(airportCode)

	url := repo.AMSRestServiceURL + "/" + repo.AMSAirport + "/" + resourceType

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Get Resource Client: Could not create request: %s\n", err))
		return nil
	}

	req.Header.Set("Authorization", repo.AMSToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Get Resource Client: error making http request: %s\n", err))
		return nil
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Get Resource Client: could not read response body: %s\n", err))
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
		globals.Logger.Error(fmt.Sprintf("Native API Test Client: could not create request: %s\n", err))
		return false
	}

	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("SOAPAction", "http://www.sita.aero/ams6-xml-api-webservice/IAMSIntegrationService/GetAirports")

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		globals.Logger.Error(fmt.Sprintf("Native API Test Client: error making http request: %s\n", err))
		return false
	}

	return true
}

func testRestAPIConnectivity(airportCode string) bool {
	repo := GetRepo(airportCode)

	url := repo.AMSRestServiceURL + "/" + repo.AMSAirport + "/Gates"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Test Connectivity Client: Could not create request: %s\n", err))
		return false
	}

	req.Header.Set("Authorization", repo.AMSToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		globals.Logger.Error(fmt.Sprintf("Test Connectivity Client: error making http request: %s\n", err))
		return false
	}

	_, err = io.ReadAll(res.Body)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Test Connectivity Client: could not read response body: %s\n", err))
		return false
	}

	return true
}
