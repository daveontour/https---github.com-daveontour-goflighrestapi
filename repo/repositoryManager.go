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

const xmlGetFlightsTemplateBody = `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ams6="http://www.sita.aero/ams6-xml-api-webservice">
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

	// Load the configuration from the airports.json config
	var repos models.Repositories
	err := globals.AirportsViper.Unmarshal(&repos)
	if err != nil {
		fmt.Println(err)
	}

	// Add each airport to the global list and then initialise it
	for _, v := range repos.Repositories {
		globals.RepoList = append(globals.RepoList, v)
		go initRepository(v.AMSAirport)
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

	//Make sure the required services are available and loop until they are.
	//This may occur if this service starts before AMS
	for !testNativeAPIConnectivity(airportCode) || !testRestAPIConnectivity(airportCode) {
		globals.Logger.Warn(fmt.Sprintf("AMS Webservice API or AMS RestAPI not avaiable for %s. Will try again in 8 seconds", airportCode))
		time.Sleep(8 * time.Second)
	}

	//Clear the MSMQ notifiaction queue if using MSMQ
	clearMSMQ(airportCode)

	// Get the resources from the RestAPI Server
	populateResourceMaps(airportCode)

	// Schedule the periodic updates and start listening
	// The listening mechanism is blocking, so has to be a "go" function
	go MaintainRepository(airportCode, false)

	// Initialise the airport repository
	loadRepositoryOnStartup(airportCode)
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

func MaintainRepository(airportCode string, perfTest bool) {

	if !perfTest {
		// Schedule the regular refresh, the scheduler is blocking, do it's in a "go"  routine
		go scheduleUpdates(airportCode)
	}

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
					go UpdateFlightEntry(message, false)
					queue.Close()
					continue ReconnectMSMQ
				}
				if strings.Contains(message, "FlightCreatedNotification") {
					go createFlightEntry(message)
					queue.Close()
					continue ReconnectMSMQ
				}
				if strings.Contains(message, "FlightDeletedNotification") {
					go deleteFlightEntry(message)
					queue.Close()
					continue ReconnectMSMQ
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

		// Declare the Exchange which the system will try to match if it exists or create if it doesn't
		err = ch.ExchangeDeclare(
			repo.RabbitMQExchange, // exchange
			"topic",               // routing key
			true,                  // durable
			false,                 // auto-deleted
			false,                 // internal
			false,                 // no-wait
			nil,                   // arguments
		)
		failOnError(err, "Failed to declare an exchange")

		//the session queue that will receive the messages from the Topic publisher
		q, err := ch.QueueDeclare(
			"",    // name
			false, // durable
			false, // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		failOnError(err, "Failed to declare the listening queue")

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
			i := 1
			for d := range msgs {
				globals.Logger.Debug("Rabbit Message Received")
				fmt.Println("Rabbit Message Received ", i)
				i++
				message := string(d.Body[:])

				globals.Logger.Debug(fmt.Sprintf("Received Message length %d\n", len(message)))

				if strings.Contains(message, "FlightUpdatedNotification") {
					go UpdateFlightEntry(message, false)
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
	//s.Every(1).Millisecond().LimitRunsTo(1).Do(func() { loadRepositoryOnStartup(airportCode) })

	globals.Logger.Info(fmt.Sprintf("Regular updates of the repository have been scheduled at %s for every %v hours", startTimeStr, globals.ConfigViper.GetString("ScheduleUpdateJobIntervalInHours")))

	s.StartBlocking()
}
func loadRepositoryOnStartup(airportCode string) {

	updateRepository(airportCode)

	// Schedule the automated scheduled pushes to for defined endpoints
	go SchedulePushes(airportCode, false)

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
			upadateAllocation(flight, airportCode, false)
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

func clearMSMQ(airportCode string) {

	repo := GetRepo(airportCode)

	if repo.ListenerType == "MSMQ" {
		// Purge the listening queue first before doing the Initializarion of the repository
		opts := []msmq.QueueInfoOption{
			msmq.WithPathName(repo.NotificationListenerQueue),
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
