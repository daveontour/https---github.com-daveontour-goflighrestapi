package main

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

type ResourceAllocationMap struct {
	Resource             FixedResource
	FlightAllocationsMap map[string]AllocationItem
}
type ParameterValuePair struct {
	Parameter string `json:"Parameter,omitempty"`
	Value     string `json:"Value,omitempty"`
}

type PropertyValuePair struct {
	Text         string `xml:",chardata"`
	PropertyName string `xml:"propertyName,attr"`
}

type ServiceConfig struct {
	ServiceName                      string `json:"ServiceName"`
	ServicDisplayName                string `json:"ServiceDisplayName"`
	ServiceDescription               string `json:"ServiceDescription"`
	ServiceIPPort                    string `json:"ServiceIPport"`
	ScheduleUpdateJob                string `json:"ScheduleUpdateJob"`
	ScheduleUpdateJobIntervalInHours int    `json:"ScheduleUpdateJobIntervalInHours"`
	DebugService                     bool   `json:"DebugService"`
	UseHTTPS                         bool   `json:"UseHTTPS"`
	UseHTTPSUntrusted                bool   `json:"UseHTTPSUntrusted"`
	KeyFile                          string `json:"KeyFile"`
	CertFile                         string `json:"CertFile"`
	TestHTTPServer                   bool   `json:"TestHTTPServer"`
	LogFile                          string `json:"LogFile"`
	RequestLogFile                   string `json:"RequestLogFile"`
	MaxLogFileSizeInMB               int    `json:"MaxLogFileSizeInMB"`
	MaxNumberLogFiles                int    `json:"MaxNumberLogFiles"`
	EnableMetrics                    bool   `json:"EnableMetrics"`
	MetricsLogFile                   string `json:"MetricsLogFile"`
	AdminToken                       string `json:"AdminToken"`
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
	ParameterChange          []string
}

type Users struct {
	Users []UserProfile `json:"users"`
}

type Repository struct {
	AMSAirport                string `json:"AMSAirport"`
	AMSSOAPServiceURL         string `json:"AMSSOAPServiceURL"`
	AMSRestServiceURL         string `json:"AMSRestServiceURL"`
	AMSToken                  string `json:"AMSToken"`
	WindowMinInDaysFromNow    int    `json:"FlightSDOWindowMinimumInDaysFromNow"`
	WindowMaxInDaysFromNow    int    `json:"FlightSDOWindowMaximumInDaysFromNow"`
	ListenerType              string `json:"ListenerType"`
	NotificationListenerQueue string `json:"NotificationListenerQueue"`
	LoadFlightChunkSizeInDays int    `json:"LoadFlightChunkSizeInDays"`
	Flights                   map[string]Flight
	CurrentLowerLimit         time.Time
	CurrentUpperLimit         time.Time
	CheckInAllocationMap      map[string]ResourceAllocationMap
	StandAllocationMap        map[string]ResourceAllocationMap
	GateAllocationMap         map[string]ResourceAllocationMap
	CarouselAllocationMap     map[string]ResourceAllocationMap
	ChuteAllocationMap        map[string]ResourceAllocationMap
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

func (r *Repository) updateLowerLimit(t time.Time) {
	r.CurrentLowerLimit = t
}
func (r *Repository) updateUpperLimit(t time.Time) {
	r.CurrentUpperLimit = t
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
