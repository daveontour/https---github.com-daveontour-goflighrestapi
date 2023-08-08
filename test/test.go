package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/models"
	"flightresourcerestapi/repo"
	"flightresourcerestapi/timeservice"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
)

const flightUpdateBody = `<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" apiVersion="2.13" user="Movement Time Trigger" timestamp="2023-08-07T09:58:00" xmlns="http://www.sita.aero/ams6-xml-api-messages">
  <Content>
    <FlightUpdatedNotification>
      <Flight>
        <DataVersion xmlns="http://www.sita.aero/ams6-xml-api-datatypes">186</DataVersion>
        <FlightId xmlns="http://www.sita.aero/ams6-xml-api-datatypes">
          <FlightKind>Departure</FlightKind>
          <AirlineDesignator codeContext="IATA">%s</AirlineDesignator>
          <FlightNumber>%d</FlightNumber>
          <ScheduledDate>%s</ScheduledDate>
          <AirportCode codeContext="IATA">%s</AirportCode>
        </FlightId>
        <FlightState xmlns="http://www.sita.aero/ams6-xml-api-datatypes">
          <ScheduledTime>%s</ScheduledTime>
          <AircraftType>
            <AircraftTypeId>
              <AircraftTypeCode codeContext="IATA">733</AircraftTypeCode>
              <AircraftTypeCode codeContext="ICAO">B733</AircraftTypeCode>
            </AircraftTypeId>
            <Value propertyName="Name">Boeing 737</Value>
          </AircraftType>
          <Aircraft>
            <AircraftId>
              <Registration>%s</Registration>
            </AircraftId>
            <Value propertyName="IsRetired">false</Value>
          </Aircraft>
          <Route customsType="International">
            <ViaPoints>
              <RouteViaPoint sequenceNumber="0">
                <AirportCode codeContext="IATA">BKK</AirportCode>
                <AirportCode codeContext="ICAO">VTBS</AirportCode>
              </RouteViaPoint>
            </ViaPoints>
          </Route>
          <Value propertyName="CheckInGroupIsManuallySet">false</Value>
          <Value propertyName="FlightUniqueID">DEP_%d</Value>
          <Value propertyName="I--G_EstimatedElapsedTime">0</Value>
          <Value propertyName="S---_Status">SCH - Scheduled</Value>
          <Value propertyName="Il--_TotalBaggageCount">0</Value>
          <Value propertyName="Il--_TransferBaggageCount">0</Value>
          <Value propertyName="S---_Terminal">T1</Value>
          <Value propertyName="Il--_TotalBookedFirstPax">0</Value>
          <Value propertyName="Il--_TotalBookedBusinessPax">0</Value>
          <Value propertyName="Il--_TotalBookedEconomyPax">0</Value>
          <Value propertyName="S---_Qualifier">J-Scheduled PAX Normal Service</Value>
          <Value propertyName="Il--_TotalMalePax">0</Value>
          <Value propertyName="Il--_TotalFemalePax">0</Value>
          <Value propertyName="Il--_TotalAdultPax">0</Value>
          <Value propertyName="Il--_TotalChildrenPax">0</Value>
          <Value propertyName="Il--_TotalInfantPax">0</Value>
          <Value propertyName="Il--_TotalCrew">0</Value>
          <Value propertyName="S--G_ScheduledAircraftType">733</Value>
          <Value propertyName="de--_TargetOffBlock">2023-08-06T10:39:00</Value>
          <Value propertyName="Il--_TotalJumpSeats">0</Value>
          <Value propertyName="I--G_AircraftFirstPax">0</Value>
          <Value propertyName="I--G_AircraftBusinessPax">0</Value>
          <Value propertyName="I--G_AircraftEconomyPax">0</Value>
          <Value propertyName="S--G_SupplementaryQualifier">PAX</Value>
          <Value propertyName="S---_RemarkDescription">
          </Value>
          <Value propertyName="Dl--_TotalCabinBaggageLoad">0</Value>
          <Value propertyName="NonADACapron">false</Value>
          <Value propertyName="Dl--_TransferCargoLoad">0</Value>
          <Value propertyName="Dl--_TransferMailLoad">0</Value>
          <Value propertyName="Sh--_GroundHandler">EAS</Value>
          <Value propertyName="Il--_TCIPax">0</Value>
          <Value propertyName="B--G_PSMReceived">false</Value>
          <Value propertyName="de--_MostConfidentDepartureTime">2023-08-07T10:03:00</Value>
          <Value propertyName="Il--_TotalFirstPax_Source04">0</Value>
          <Value propertyName="Il--_TotalBusinessPax_Source04">0</Value>
          <Value propertyName="Il--_TotalEconomyPax_Source04">0</Value>
          <Value propertyName="S--G_FlightType_Output05">P</Value>
          <Value propertyName="S--G_FlightType_Output10">O</Value>
          <Value propertyName="S--G_FlightType_Output14">O</Value>
          <Value propertyName="S--G_Qualifier_Output10">J</Value>
          <Value propertyName="S--G_Qualifier_Output13">J-Scheduled PAX Normal Service</Value>
          <Value propertyName="S--G_Qualifier_Output14">J-Scheduled PAX Normal Service</Value>
          <Value propertyName="S--G_OperationalRemark_Output07">A</Value>
          <Value propertyName="S--G_OperationalRemark_Output09">O</Value>
          <Value propertyName="S--G_OperationalRemark_Output13">Scheduled</Value>
          <Value propertyName="S--G_OperationalRemark_Output15">O</Value>
          <Value propertyName="S--G_OperationalNatureCode_Output14">PAX</Value>
          <Value propertyName="B---_Blacklist Flight">false</Value>
          <Value propertyName="Il--_CockpitCrew">0</Value>
          <Value propertyName="Il--_CabinCrewMale">0</Value>
          <Value propertyName="Il--_CabinCrewFemale">0</Value>
          <Value propertyName="Dl--_TransitDeadLoad">0</Value>
          <Value propertyName="Dl--_LoadedCargoWeight">0</Value>
          <Value propertyName="Dl--_LoadedMailWeight">0</Value>
          <Value propertyName="Il--_TotalBookedPax">0</Value>
          <Value propertyName="Il--_TotalCabinCrew">0</Value>
          <Value propertyName="Il--_InfantTransitPax">0</Value>
          <Value propertyName="Dl--_TransitCargoLoad">0</Value>
          <Value propertyName="Dl--_TransitMailLoad">0</Value>
          <Value propertyName="S--G_Qualifier_Output09">J</Value>
          <Value propertyName="B--G_DataTransmitFlag_Output07">true</Value>
          <Value propertyName="S---_CBPFlights">false</Value>
          <Value propertyName="S--G_DepartureStandType">Contact</Value>
          <Value propertyName="B--G_AdHocFlight">false</Value>
          <Value propertyName="S---_AdhocFlightStatus" />
          <Value propertyName="I--G_ReturnCount">0</Value>
          <Value propertyName="S--G_StopType">Turnaround</Value>
          <Value propertyName="B---_NoChangeAllowed">false</Value>
          <Value propertyName="B--G_BillingEligibility">false</Value>
          <Value propertyName="de--_LastKnownTargetOffBlock">2023-08-06T10:39:00</Value>
          <Value propertyName="B--G_PublishedToBilling">false</Value>
          <Value propertyName="B--G_PublishedToERPATC">false</Value>
          <Value propertyName="Il--_TotalDeadHeadCrew">0</Value>
          <Value propertyName="Dl--_TotalDeadLoad">0</Value>
          <Value propertyName="Dl--_TransitBaggageLoad">0</Value>
          <Value propertyName="S--G_AirlineCreditStatus">Credit</Value>
          <Value propertyName="Il--_AdultTransitPax">0</Value>
          <Value propertyName="Il--_ChildrenTransitPax">0</Value>
          <Value propertyName="Il--_MaleTransitPax">0</Value>
          <Value propertyName="Il--_TransitBusinessPax">0</Value>
          <Value propertyName="Il--_TransitEconomyPax">0</Value>
          <Value propertyName="Il--_FemaleTransitPax">0</Value>
          <Value propertyName="Dl--_LoadedBaggageWeight">0</Value>
          <Value propertyName="Il--_TransitFirstPax">0</Value>
          <Value propertyName="B--G_DataTransmitFlag_Output13">false</Value>
          <Value propertyName="I--G_ScheduledTurnaroundTime">40</Value>
          <Value propertyName="S---_AirlineName">ME/MEA Middle East Airlines</Value>
          <Value propertyName="Original Flight Number">ME6521</Value>
          <Value propertyName="S--G_Qualifier_Source00">J-Scheduled PAX Normal Service</Value>
          <Value propertyName="d--G_LastUpdateTime">2023-08-06T11:57:00</Value>
          <Value propertyName="S--G_PTMReceiptIndicator">No</Value>
          <Value propertyName="Il--_TotalBussedTransferBaggageCount">0</Value>
          <Value propertyName="S--G_PRLReceiptIndicator">No</Value>
          <Value propertyName="DWIterationCount-Dep">0</Value>
          <Value propertyName="Il--_TotalBaggageCount_Source02">0</Value>
          <Value propertyName="S--G_StandArea">Apron 1</Value>
          <Value propertyName="B--G_HighRisk">false</Value>
          <Value propertyName="IterationCountCheck-Dep">0</Value>
          <Value propertyName="B---_AdditionalCounterRequest">false</Value>
          <Value propertyName="Clear Target Time Flag">false</Value>
          <Value propertyName="Route Discrepancy">false</Value>
          <Value propertyName="B--G_BaggageResourceUnAllocationIndicator">false</Value>
          <Value propertyName="Il--_TotalEconomyPax_Source20">0</Value>
          <Value propertyName="Il--_TotalPremiumEconomyPax_Source20">0</Value>
          <Value propertyName="Il--_TotalFirstPax_Source20">0</Value>
          <Value propertyName="Il--_TotalBusinessPax_Source20">0</Value>
          <Value propertyName="Il--_TotalJoiningPax_Source20">0</Value>
          <Value propertyName="Il--_TotalTransferPax_Source20">0</Value>
          <Value propertyName="S--G_CheckInCounterType">Dedicated</Value>
          <Value propertyName="PrevStandTemp">103</Value>
          <Value propertyName="LinkingAlert">false</Value>
          <Value propertyName="Il--_TotalBookedPremiumEconomyPax">0</Value>
          <Value propertyName="DataLocked">false</Value>
          <Value propertyName="Stand">103</Value>
          <TableValue propertyName="Td--_DelayCodes_Old" />
          <TableValue propertyName="TS--_PassengerServices" />
          <TableValue propertyName="Tl--_BussedTransferPax_old" />
          <TableValue propertyName="Te--_CounterUsageData" />
          <TableValue propertyName="Tl--_PRLTransferLoads" />
          <TableValue propertyName="T---_ResourceChange" />
          <TableValue propertyName="Ts--_Services" />
          <TableValue propertyName="Tl--_TransferLoads" />
          <TableValue propertyName="Td--_DelayCodes" />
          <TableValue propertyName="Tl--_AdditionalLoads" />
          <TableValue propertyName="T---_TowDetail" />
          <TableValue propertyName="Tl--_BussedTransferPax" />
          <StandSlots>
            <StandSlot>
              <Value propertyName="StartTime">2023-08-06T09:59:00</Value>
              <Value propertyName="EndTime">2023-08-07T10:03:00</Value>
              <Stand>
                <Value propertyName="Name">103</Value>
                <Value propertyName="ExternalName">103</Value>
                <Area>
                  <Value propertyName="Name">Apron 1</Value>
                </Area>
              </Stand>
            </StandSlot>
          </StandSlots>
          <GateSlots>
            <GateSlot>
              <Value propertyName="StartTime">2023-08-07T09:33:00</Value>
              <Value propertyName="EndTime">2023-08-07T10:03:00</Value>
              <Value propertyName="Category">departure</Value>
            </GateSlot>
          </GateSlots>
          <CheckInSlots>
            <CheckInSlot>
              <Value propertyName="StartTime">2023-08-06T07:39:00</Value>
              <Value propertyName="EndTime">2023-08-06T09:39:00</Value>
              <Value propertyName="Category">Business</Value>
            </CheckInSlot>
            <CheckInSlot>
              <Value propertyName="StartTime">2023-08-06T07:39:00</Value>
              <Value propertyName="EndTime">2023-08-06T09:39:00</Value>
              <Value propertyName="Category">Economy</Value>
            </CheckInSlot>
            <CheckInSlot>
              <Value propertyName="StartTime">2023-08-06T07:39:00</Value>
              <Value propertyName="EndTime">2023-08-06T09:39:00</Value>
              <Value propertyName="Category">Economy</Value>
            </CheckInSlot>
          </CheckInSlots>
        </FlightState>
        <FlightChanges xmlns="http://www.sita.aero/ams6-xml-api-datatypes">
          <Change propertyName="de--_MostConfidentDepartureTime">
            <OldValue>2023-08-07T09:57:00</OldValue>
            <NewValue>2023-08-07T10:03:00</NewValue>
          </Change>
          <StandSlotsChange>
            <OldValue>
              <StandSlot>
                <Value propertyName="StartTime">2023-08-06T09:59:00</Value>
                <Value propertyName="EndTime">2023-08-07T09:57:00</Value>
                <Stand>
                  <Value propertyName="Name">103</Value>
                  <Value propertyName="ExternalName">103</Value>
                  <Area>
                    <Value propertyName="Name">Apron 1</Value>
                  </Area>
                </Stand>
              </StandSlot>
            </OldValue>
            <NewValue>
              <StandSlot>
                <Value propertyName="StartTime">2023-08-06T09:59:00</Value>
                <Value propertyName="EndTime">2023-08-07T10:03:00</Value>
                <Stand>
                  <Value propertyName="Name">103</Value>
                  <Value propertyName="ExternalName">103</Value>
                  <Area>
                    <Value propertyName="Name">Apron 1</Value>
                  </Area>
                </Stand>
              </StandSlot>
            </NewValue>
          </StandSlotsChange>
          <GateSlotsChange>
            <OldValue>
              <GateSlot>
                <Value propertyName="StartTime">2023-08-07T09:27:00</Value>
                <Value propertyName="EndTime">2023-08-07T09:57:00</Value>
                <Value propertyName="Category">departure</Value>
              </GateSlot>
            </OldValue>
            <NewValue>
              <GateSlot>
                <Value propertyName="StartTime">2023-08-07T09:33:00</Value>
                <Value propertyName="EndTime">2023-08-07T10:03:00</Value>
                <Value propertyName="Category">departure</Value>
              </GateSlot>
            </NewValue>
          </GateSlotsChange>
        </FlightChanges>
      </Flight>
    </FlightUpdatedNotification>
  </Content>
</Envelope>
`

type AutoGenerated struct {
	TestConfig struct {
		Repository   models.Repository `json:"Repository"`
		CheckinAreas []struct {
			Area   string `json:"Area"`
			Number int    `json:"Number"`
		} `json:"CheckinAreas"`
		GateAreas []struct {
			Area   string `json:"Area"`
			Number int    `json:"Number"`
		} `json:"GateAreas"`
		StandAreas []struct {
			Area   string `json:"Area"`
			Number int    `json:"Number"`
		} `json:"StandAreas"`
		CarouselAreas []struct {
			Area   string `json:"Area"`
			Number int    `json:"Number"`
		} `json:"CarouselAreas"`
		ChuteAreas []struct {
			Area   string `json:"Area"`
			Number int    `json:"Number"`
		} `json:"ChuteAreas"`
		Airlines []string `json:"Airlines"`
	} `json:"TestConfig"`
}

func PerfTestInit(nf int) {

	exe, err0 := os.Executable()
	if err0 != nil {
		panic(err0)
	}
	exPath := filepath.Dir(exe)
	testViper := viper.New()

	testViper.SetConfigName("test") // name of config file (without extension)
	testViper.SetConfigType("json") // REQUIRED if the config file does not have the extension in the name
	testViper.AddConfigPath(".")    // optionally look for config in the working directory
	testViper.AddConfigPath(exPath)
	if err := testViper.ReadInConfig(); err != nil {
		globals.Logger.Fatal("Could Not Read test.json config file")
	}

	var config = AutoGenerated{}
	if err := testViper.Unmarshal(&config); err != nil {
		fmt.Println("Error reading test config file")
		return
	}

	globals.RepoList = append(globals.RepoList, config.TestConfig.Repository)

	rep := repo.GetRepo(config.TestConfig.Repository.AMSAirport)

	for _, ci := range config.TestConfig.CheckinAreas {
		addResource(ci.Area, ci.Number, "CheckIn", &rep.CheckInList)
	}
	for _, ci := range config.TestConfig.GateAreas {
		addResource(ci.Area, ci.Number, "Gate", &rep.GateList)
	}
	for _, ci := range config.TestConfig.StandAreas {
		addResource(ci.Area, ci.Number, "Stand", &rep.StandList)
	}
	for _, ci := range config.TestConfig.CarouselAreas {
		addResource(ci.Area, ci.Number, "Carousel", &rep.CarouselList)
	}
	for _, ci := range config.TestConfig.ChuteAreas {
		addResource(ci.Area, ci.Number, "Chute", &rep.ChuteList)
	}

	t := time.Now()

	for i := 0; i < nf; i++ {

		al := config.TestConfig.Airlines[i%len(config.TestConfig.Airlines)]
		fltNum := 1 + i

		sto := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		sto = sto.Add(time.Minute * time.Duration(i))

		st := sto.Format(timeservice.Layout)
		sd := sto.Format("2006-01-02")
		reg := fmt.Sprintf("VH-%d", i)
		msg := fmt.Sprintf(flightUpdateBody, al, fltNum, sd, config.TestConfig.Repository.AMSAirport, st, reg, 234174+i)
		fmt.Println("Sending inital message", fltNum)
		repo.UpdateFlightEntry(msg)
	}
}

func addResource(area string, num int, rtype string, arr *models.ResourceLinkedList) {

	for i := 1; i <= num; i++ {
		arr.AddNode(
			models.ResourceAllocationStruct{
				Resource: models.FixedResource{
					ResourceTypeCode: rtype,
					Name:             fmt.Sprintf("%s%d", area, i),
					Area:             area,
				},
			},
		)
	}

}

func SendUpdateMessages(nf int) {

	exe, err0 := os.Executable()
	if err0 != nil {
		panic(err0)
	}
	exPath := filepath.Dir(exe)
	testViper := viper.New()

	testViper.SetConfigName("test") // name of config file (without extension)
	testViper.SetConfigType("json") // REQUIRED if the config file does not have the extension in the name
	testViper.AddConfigPath(".")    // optionally look for config in the working directory
	testViper.AddConfigPath(exPath)
	if err := testViper.ReadInConfig(); err != nil {
		globals.Logger.Fatal("Could Not Read test.json config file")
	}

	var config = AutoGenerated{}
	if err := testViper.Unmarshal(&config); err != nil {
		fmt.Println("Error reading test config file")
		return
	}

	t := time.Now()
	for i := 0; i < nf; i++ {

		al := config.TestConfig.Airlines[i%len(config.TestConfig.Airlines)]
		fltNum := 1 + i

		sto := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		sto = sto.Add(time.Minute * time.Duration(i))

		st := sto.Format(timeservice.Layout)
		sd := sto.Format("2006-01-02")
		reg := fmt.Sprintf("VH-%d", i)
		msg := fmt.Sprintf(flightUpdateBody, al, fltNum, sd, config.TestConfig.Repository.AMSAirport, st, reg, 234174+i)
		fmt.Println("Sending Update Message", fltNum)
		repo.UpdateFlightEntry(msg)
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func rmq() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := "Hello World!"
	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	failOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
}

func publistopic() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"logs_topic", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := bodyFrom(os.Args)
	err = ch.PublishWithContext(ctx,
		"logs_topic",          // exchange
		severityFrom(os.Args), // routing key
		false,                 // mandatory
		false,                 // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	failOnError(err, "Failed to publish a message")

	log.Printf(" [x] Sent %s", body)
}

func bodyFrom(args []string) string {
	var s string
	if (len(args) < 3) || os.Args[2] == "" {
		s = "hello"
	} else {
		s = strings.Join(args[2:], " ")
	}
	return s
}

func severityFrom(args []string) string {
	var s string
	if (len(args) < 2) || os.Args[1] == "" {
		s = "anonymous.info"
	} else {
		s = os.Args[1]
	}
	return s
}

func receivetopic() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"logs_topic", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	if len(os.Args) < 2 {
		log.Printf("Usage: %s [binding_key]...", os.Args[0])
		os.Exit(0)
	}
	for _, s := range os.Args[1:] {
		log.Printf("Binding queue %s to exchange %s with routing key %s", q.Name, "logs_topic", s)
		err = ch.QueueBind(
			q.Name,       // queue name
			s,            // routing key
			"logs_topic", // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")
	}

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

	go func() {
		for d := range msgs {
			log.Printf(" [x] %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for logs. To exit press CTRL+C")
	<-forever
}
