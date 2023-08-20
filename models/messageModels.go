package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type FixedResource struct {
	ResourceTypeCode string `xml:"ResourceTypeCode"`
	Name             string `xml:"Name"`
	Area             string `xml:"Area"`
}
type FixedResources struct {
	Values []FixedResource `xml:"FixedResource"`
}

type AllocationItem struct {
	PrevNode             *AllocationItem `xml:"-" json:"-"`
	NextNode             *AllocationItem `xml:"-" json:"-"`
	ResourceID           string
	From                 time.Time
	To                   time.Time
	FlightID             string
	Direction            string
	Route                string
	AircraftType         string
	AircraftRegistration string
	AirportCode          string
	LastUpdate           time.Time
}

type AllocationResponseItem struct {
	ResourceType string `xml:"ResourceTypeCode"`
	Name         string `xml:"Name"`
	Area         string `xml:"Area"`
	AllocationItem
}

type ConfiguredResourceResponseItem struct {
	ResourceTypeCode string `xml:"ResourceTypeCode"`
	Name             string `xml:"Name"`
	Area             string `xml:"Area"`
}

type AllocationLinkedList struct {
	Head *AllocationItem
	Tail *AllocationItem
}

func (ll *AllocationLinkedList) RemoveFlightAllocations(flightID string) {
	currentNode := ll.Head

	for currentNode != nil {
		if currentNode.FlightID == flightID {

			if currentNode.PrevNode != nil {
				currentNode.PrevNode.NextNode = currentNode.NextNode
			} else {
				ll.Head = currentNode.NextNode
			}

			if currentNode.NextNode != nil {
				currentNode.NextNode.PrevNode = currentNode.PrevNode
			} else {
				ll.Tail = currentNode.PrevNode
			}

			currentNode.PrevNode = nil
			currentNode.NextNode = nil

			//return // Node found and removed, exit the function
		}

		currentNode = currentNode.NextNode
	}
}

func (ll *AllocationLinkedList) Len() int {
	currentNode := ll.Head
	count := 0

	for currentNode != nil {
		count++
		currentNode = currentNode.NextNode
	}

	return count
}
func (ll *AllocationLinkedList) AddNode(newNode AllocationItem) {

	newNode.PrevNode = ll.Tail
	newNode.NextNode = nil

	if ll.Tail != nil {
		ll.Tail.NextNode = &newNode
	}

	ll.Tail = &newNode

	if ll.Head == nil {
		ll.Head = &newNode
	}
}

type ResourceAllocationStruct struct {
	PrevNode              *ResourceAllocationStruct
	NextNode              *ResourceAllocationStruct
	Resource              FixedResource
	FlightAllocationsList AllocationLinkedList
}

type ParameterValuePair struct {
	Parameter string `json:"Parameter,omitempty"`
	Value     string `json:"Value,omitempty"`
}

type PropertyValuePair struct {
	Text         string `xml:",chardata"`
	PropertyName string `xml:"propertyName,attr"`
}
type MetricsReport struct {
	Airport                     string
	NumberOfFlights             int
	NumberOfCheckins            int
	NumberOfCheckinAllocations  int
	NumberOfGates               int
	NumberOfGateAllocations     int
	NumberOfStands              int
	NumberOfStandAllocations    int
	NumberOfCarousels           int
	NumberOfCarouselAllocations int
	NumberOfChutes              int
	NumberOfChuteAllocations    int
	MemAllocMB                  int
	MemHeapAllocMB              int
	MemTotaAllocMB              int
	MemSysMB                    int
	MemNumGC                    int
}

type UserProfile struct {
	Enabled                      bool                     `json:"Enabled"`
	UserName                     string                   `json:"UserName"`
	Key                          string                   `json:"Key"`
	AllowedAirports              []string                 `json:"AllowedAirports"`
	AllowedAirlines              []string                 `json:"AllowedAirlines"`
	AllowedCustomFields          []string                 `json:"AllowedCustomFields"`
	DefaultAirline               string                   `json:"DefaultAirline"`
	DefaultQueryableCustomFields []ParameterValuePair     `json:"DefaultQueryableCustomFields"`
	UserPushSubscriptions        []UserPushSubscription   `json:"UserPushSubscriptions"`
	UserChangeSubscriptions      []UserChangeSubscription `json:"UserChangeSubscriptions"`
}

type UserPushSubscription struct {
	Enabled               bool
	EnableInDemoMode      bool
	PushOnStartUp         bool
	Airport               string
	DestinationURL        string
	HeaderParameters      []ParameterValuePair
	SubscriptionType      string
	Time                  string
	ReptitionHours        int
	ReptitionMinutes      int
	Airline               string
	From                  int
	To                    int
	QueryableCustomFields []ParameterValuePair
	ResourceType          string
	ResourceID            string
	Route                 string
	Direction             string
	TrustBadCertificates  bool
}

type UserChangeSubscription struct {
	Enabled                  bool
	Airport                  string
	DestinationURL           string
	HeaderParameters         []ParameterValuePair
	CheckInChange            bool
	GateChange               bool
	StandChange              bool
	CarouselChange           bool
	ChuteChange              bool
	AircraftTypeOrRegoChange bool
	EventChange              bool
	CreateFlight             bool
	DeleteFlight             bool
	UpdateFlight             bool
	All                      bool
	ParameterChange          []string
	TrustBadCertificates     bool
}

type Users struct {
	Users []UserProfile `json:"users"`
}

type ResourceLinkedList struct {
	Head *ResourceAllocationStruct
	Tail *ResourceAllocationStruct
}

func (ll *ResourceLinkedList) AddAllocation(node AllocationItem) {
	currentNode := ll.Head

	for currentNode != nil {
		if currentNode.Resource.Name == node.ResourceID {
			currentNode.FlightAllocationsList.AddNode(node)
			break
		}
		currentNode = currentNode.NextNode
	}
}
func (ll *ResourceLinkedList) AddNodes(nodes []FixedResource) {
	for _, node := range nodes {
		newNode := ResourceAllocationStruct{Resource: node}
		ll.AddNode(newNode)
	}
}
func (ll *ResourceLinkedList) AddNode(newNode ResourceAllocationStruct) {

	newNode.PrevNode = ll.Tail
	newNode.NextNode = nil

	if ll.Tail != nil {
		ll.Tail.NextNode = &newNode
	}

	ll.Tail = &newNode

	if ll.Head == nil {
		ll.Head = &newNode
	}
}
func (ll *ResourceLinkedList) RemoveFlightAllocation(flightID string) {
	currentNode := ll.Head

	for currentNode != nil {
		currentNode.FlightAllocationsList.RemoveFlightAllocations(flightID)
		currentNode = currentNode.NextNode
	}
}
func (ll *ResourceLinkedList) Len() int {
	currentNode := ll.Head
	count := 0

	for currentNode != nil {
		count++
		currentNode = currentNode.NextNode
	}

	return count
}
func (ll *ResourceLinkedList) NumberOfFlightAllocations() (n int) {
	currentNode := ll.Head

	for currentNode != nil {
		n = n + currentNode.FlightAllocationsList.Len()
		currentNode = currentNode.NextNode
	}
	return
}

// FlightLinkedList represents the doubly linked list.
type FlightLinkedList struct {
	Head *Flight
	Tail *Flight
}

func (ll *FlightLinkedList) RemoveNode(removeNode Flight) {
	currentNode := ll.Head

	for currentNode != nil {
		if currentNode.GetFlightID() == removeNode.GetFlightID() {
			if currentNode.PrevNode != nil {
				currentNode.PrevNode.NextNode = currentNode.NextNode
			} else {
				ll.Head = currentNode.NextNode
			}

			if currentNode.NextNode != nil {
				currentNode.NextNode.PrevNode = currentNode.PrevNode
			} else {
				ll.Tail = currentNode.PrevNode
			}

			currentNode.PrevNode = nil
			currentNode.NextNode = nil

			return // Node found and removed, exit the function
		}

		currentNode = currentNode.NextNode
	}
}
func (ll *FlightLinkedList) Len() int {
	currentNode := ll.Head
	count := 0

	for currentNode != nil {
		count++
		currentNode = currentNode.NextNode
	}

	return count
}

// AddNode adds a new node to the end of the doubly linked list.
func (ll *FlightLinkedList) AddNode(newNode Flight) {

	newNode.PrevNode = ll.Tail
	newNode.NextNode = nil

	if ll.Tail != nil {
		ll.Tail.NextNode = &newNode
	}

	ll.Tail = &newNode

	if ll.Head == nil {
		ll.Head = &newNode
	}
}
func (ll *FlightLinkedList) ReplaceOrAddNode(node Flight) {
	currentNode := ll.Head

	for currentNode != nil {
		if currentNode.GetFlightID() == node.GetFlightID() {
			// Replace the entire node
			node.PrevNode = currentNode.PrevNode
			node.NextNode = currentNode.NextNode

			if currentNode.PrevNode != nil {
				currentNode.PrevNode.NextNode = &node
			} else {
				ll.Head = &node
			}

			if currentNode.NextNode != nil {
				currentNode.NextNode.PrevNode = &node
			} else {
				ll.Tail = &node
			}

			currentNode.PrevNode = nil
			currentNode.NextNode = nil

			return // Node found and replaced, exit the function
		}
		currentNode = currentNode.NextNode
	}

	ll.AddNode(node)
}
func (ll *FlightLinkedList) RemoveExpiredNode(from time.Time) {
	currentNode := ll.Head

	for currentNode != nil {
		if currentNode.GetSDO().Before(from) {
			if currentNode.PrevNode != nil {
				currentNode.PrevNode.NextNode = currentNode.NextNode
			} else {
				ll.Head = currentNode.NextNode
			}

			if currentNode.NextNode != nil {
				currentNode.NextNode.PrevNode = currentNode.PrevNode
			} else {
				ll.Tail = currentNode.PrevNode
			}

			currentNode.PrevNode = nil
			currentNode.NextNode = nil

			return // Node found and removed, exit the function
		}

		currentNode = currentNode.NextNode
	}
}

type Repository struct {
	AMSAirport                          string `json:"AMSAirport"`
	AMSSOAPServiceURL                   string `json:"AMSSOAPServiceURL"`
	AMSRestServiceURL                   string `json:"AMSRestServiceURL"`
	AMSToken                            string `json:"AMSToken"`
	FlightSDOWindowMinimumInDaysFromNow int    `json:"FlightSDOWindowMinimumInDaysFromNow"`
	FlightSDOWindowMaximumInDaysFromNow int    `json:"FlightSDOWindowMaximumInDaysFromNow"`
	ListenerType                        string `json:"ListenerType"`
	RabbitMQConnectionString            string `json:"RabbitMQConnectionString"`
	RabbitMQExchange                    string `json:"RabbitMQExchange"`
	RabbitMQTopic                       string `json:"RabbitMQTopic"`
	NotificationListenerQueue           string `json:"NotificationListenerQueue"`
	LoadFlightChunkSizeInDays           int    `json:"LoadFlightChunkSizeInDays"`
	FlightLinkedList                    FlightLinkedList
	CurrentLowerLimit                   time.Time
	CurrentUpperLimit                   time.Time
	CheckInList                         ResourceLinkedList
	StandList                           ResourceLinkedList
	GateList                            ResourceLinkedList
	CarouselList                        ResourceLinkedList
	ChuteList                           ResourceLinkedList
}

func (r *Repository) RemoveFlightAllocation(flightID string) {
	r.CheckInList.RemoveFlightAllocation(flightID)
	r.GateList.RemoveFlightAllocation(flightID)
	r.StandList.RemoveFlightAllocation(flightID)
	r.CarouselList.RemoveFlightAllocation(flightID)
	r.ChuteList.RemoveFlightAllocation(flightID)
}
func (r *Repository) UpdateLowerLimit(t time.Time) {
	r.CurrentLowerLimit = t
}
func (r *Repository) UpdateUpperLimit(t time.Time) {
	r.CurrentUpperLimit = t
}

type Repositories struct {
	Repositories []Repository `json:"airports"`
}
type Request struct {
	Direction                  string
	Airline                    string
	FltNum                     string
	From                       string
	To                         string
	UpdatedSince               string
	Route                      string
	UserProfile                UserProfile
	PresentQueryableParameters []ParameterValuePair
}
type Response struct {
	User             string               `json:"User,omitempty"`
	AirportCode      string               `json:"AirportCode,omitempty"`
	Route            string               `json:"Route,omitempty"`
	From             string               `json:"FlightsFrom,omitempty"`
	To               string               `json:"FlightsTo,omitempty"`
	Airline          string               `json:"Airline,omitempty"`
	Flight           string               `json:"Flight,omitempty"`
	FromResource     string               `json:"ResourcessFrom,omitempty"`
	ToResource       string               `json:"ResourceTo,omitempty"`
	NumberOfFlights  int                  `json:"NumberOfFlights,omitempty"`
	Direction        string               `json:"Direction,omitempty"`
	CustomFieldQuery []ParameterValuePair `json:"CustomFieldQueries,omitempty"`
	Warnings         []string             `json:"Warnings,omitempty"`
	Errors           []string             `json:"Errors,omitempty"`
	Flights          []Flight             `json:"Flights,omitempty"`
}

type ResourceResponse struct {
	User                string                           `json:"User,omitempty"`
	AirportCode         string                           `json:"AirportCode,omitempty"`
	From                string                           `json:"FlightsFrom,omitempty"`
	To                  string                           `json:"FlightsTo,omitempty"`
	NumberOfFlights     int                              `json:"NumberOfFlights,omitempty"`
	FromResource        string                           `json:"ResourcessFrom,omitempty"`
	ToResource          string                           `json:"ResourceTo,omitempty"`
	Direction           string                           `json:"Direction,omitempty"`
	CustomFieldQuery    []ParameterValuePair             `json:"CustomFieldQueries,omitempty"`
	Warnings            []string                         `json:"Warnings,omitempty"`
	Errors              []string                         `json:"Errors,omitempty"`
	ResourceType        string                           `json:"ResourceType,omitempty"`
	ResourceID          string                           `json:"ResourceID,omitempty"`
	FlightID            string                           `json:"FlightID,omitempty"`
	Airline             string                           `json:"Airline,omitempty"`
	Allocations         []AllocationResponseItem         `json:"Allocations,omitempty"`
	ConfiguredResources []ConfiguredResourceResponseItem `json:"ConfiguredResources,omitempty"`
}

func (r *Response) AddWarning(w string) {
	r.Warnings = append(r.Warnings, w)
}
func (r *Response) AddError(w string) {
	r.Errors = append(r.Errors, w)
}
func (d AllocationResponseItem) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	st, _ := json.Marshal(d.ResourceType)
	sb.WriteString(fmt.Sprintf("\"ResourceType\":%s,", string(st)))
	st2, _ := json.Marshal(d.Name)
	sb.WriteString(fmt.Sprintf("\"Name\":%s,", string(st2)))
	st3, _ := json.Marshal(d.Area)
	sb.WriteString(fmt.Sprintf("\"Area\":%s,", string(st3)))

	st4, _ := json.Marshal(d.AllocationItem.From)
	sb.WriteString(fmt.Sprintf("\"AllocationStart\":%s,", string(st4)))
	st5, _ := json.Marshal(d.AllocationItem.To)
	sb.WriteString(fmt.Sprintf("\"AllocationEnd\":%s,", string(st5)))

	sb.WriteString("\"Flight\": {")

	f1, _ := json.Marshal(d.AllocationItem.FlightID)
	sb.WriteString(fmt.Sprintf("\"FlightID\":%s,", string(f1)))

	f2, _ := json.Marshal(d.AllocationItem.Direction)
	sb.WriteString(fmt.Sprintf("\"Direction\":%s,", string(f2)))

	f3, _ := json.Marshal(d.AllocationItem.Route)
	sb.WriteString(fmt.Sprintf("\"Route\":%s,", string(f3)))

	if d.AllocationItem.AircraftRegistration != "" {
		f4, _ := json.Marshal(d.AllocationItem.AircraftRegistration)
		sb.WriteString(fmt.Sprintf("\"AircraftRegistration\":%s,", string(f4)))
	}

	if d.AllocationItem.AircraftType != "" {
		f5, _ := json.Marshal(d.AllocationItem.AircraftType)
		sb.WriteString(fmt.Sprintf("\"AircraftType\":%s", string(f5)))
	}
	sb.WriteString(" },")

	s := CleanJSON(sb)

	return []byte(s), nil
}

func CleanJSON(sb strings.Builder) string {

	s := sb.String()
	if last := len(s) - 1; last >= 0 && s[last] == ',' {
		s = s[:last]
	}

	s = s + "}"

	return s
}

type GetFlightsError struct {
	StatusCode int
	Err        error
}

func (r *GetFlightsError) Error() string {
	return fmt.Sprintf("status %d: err %v", r.StatusCode, r.Err)
}

type ChangePushJob struct {
	Sub    UserChangeSubscription
	Flight Flight
}
type SchedulePushJob struct {
	Sub       UserPushSubscription
	UserToken string
	UserName  string
}
