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

type ServiceConfig struct {
	ServiceName                      string `json:"servicename"`
	ServicDisplayName                string `json:"servicedisplayname"`
	ServiceDescription               string `json:"servicedescription"`
	ServiceIPPort                    string `json:"serviceipport"`
	ScheduleUpdateJob                string `json:"scheduleUpdateJob"`
	ScheduleUpdateJobIntervalInHours int    `json:"scheduleUpdateJobIntervalInHours"`
	DebugService                     bool   `json:"debugService"`
	UseHTTPS                         bool   `json:"useHTTPS"`
	KeyFile                          string `json:"keyFile"`
	CertFile                         string `json:"certFile"`
}

type UserProfile struct {
	UserName                      string                   `json:"username"`
	Key                           string                   `json:"key"`
	AllowedAirports               []string                 `json:"allowedairports"`
	AllowedAirlines               []string                 `json:"allowedairlines"`
	AllowedCustomFields           []string                 `json:"allowedcustomfields"`
	QueryableCustomFields         []string                 `json:"queryablecustomfields"`
	DefaultAirport                string                   `json:"defaultairport"`
	OverrideAirport               string                   `json:"overrideairport"`
	DefaultAirline                string                   `json:"defaultairline"`
	OverrideAirline               string                   `json:"overrideairline"`
	DefaultQueryableCustomFields  []ParameterValuePair     `json:"defaultqueryablecustomfields"`
	OverrideQueryableCustomFields []ParameterValuePair     `json:"overridequeryablecustomfields"`
	UserPushSubscriptions         []UserPushSubscription   `json:"pushsubscriptions"`
	UserChangeSubscriptions       []UserChangeSubscription `json:"changesubscriptions"`
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
	ParameterChange          []string
}

type Users struct {
	Users []UserProfile `json:"users"`
}

type Repository struct {
	Airport               string `json:"airport"`
	URL                   string `json:"url"`
	RestURL               string `json:"resturl"`
	Token                 string `json:"token"`
	WindowMin             int    `json:"windowminimum"`
	WindowMax             int    `json:"windowmaximum"`
	ListenerType          string `json:"listenertype"`
	ListenerQueue         string `json:"listenerqueue"`
	ChunkSize             int    `json:"chunksize"`
	Flights               map[string]Flight
	CurrentLowerLimit     time.Time
	CurrentUpperLimit     time.Time
	CheckInAllocationMap  map[string]ResourceAllocationMap
	StandAllocationMap    map[string]ResourceAllocationMap
	GateAllocationMap     map[string]ResourceAllocationMap
	CarouselAllocationMap map[string]ResourceAllocationMap
	ChuteAllocationMap    map[string]ResourceAllocationMap
}

func (r *Repository) updateLowerLimit(t time.Time) {
	r.CurrentLowerLimit = t
}
func (r *Repository) updateUpperLimit(t time.Time) {
	r.CurrentUpperLimit = t
}

type Repositories struct {
	Repositories []Repository `json:"airports"`
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

func (r *Response) AddWarning(w string) {
	r.Warnings = append(r.Warnings, w)
}
func (r *Response) AddError(w string) {
	r.Errors = append(r.Errors, w)
}

type Request struct {
	Direction                  string
	Airline                    string
	From                       string
	To                         string
	UpdatedSince               string
	Route                      string
	UserProfile                UserProfile
	PresentQueryableParameters []ParameterValuePair
}

type StandAllocation struct {
	Stand  Stand
	From   time.Time
	To     time.Time
	Flight FlightId
}

type StandAllocations struct {
	Allocations []StandAllocation
}

// Resource definitions

type Area struct {
	Value []Value `xml:"Value"`
}

type Stand struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Area  Area    `xml:"Area" json:"Area,omitempty"`
}

type StandSlot struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Stand Stand   `xml:"Stand" json:"Area,omitempty"`
}
type StandSlots struct {
	StandSlot []StandSlot `xml:"StandSlot" json:"StandSlot,omitempty"`
}
type Carousel struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Area  Area    `xml:"Area" json:"Area,omitempty"`
}
type CarouselSlot struct {
	Value    []Value  `xml:"Value" json:"Slot,omitempty"`
	Carousel Carousel `xml:"Carousel" json:"Carousel,omitempty"`
}
type CarouselSlots struct {
	CarouselSlot []CarouselSlot `xml:"CarouselSlot" json:"CarouselSlot,omitempty"`
}

type Gate struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}

type GateSlot struct {
	Value []Value `xml:"Value"`
	Gate  Gate    `xml:"Gate"`
}
type GateSlots struct {
	GateSlot []GateSlot `xml:"GateSlot" json:"GateSlot,omitempty"`
}
type CheckIn struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}

func (p CheckInSlot) getResourceID() (name string, from time.Time, to time.Time) {

	const layout = "2006-01-02T15:04:05"
	loc, _ := time.LoadLocation("Local")

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
	}

	for _, v := range p.CheckIn.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}

func (p StandSlot) getResourceID() (name string, from time.Time, to time.Time) {

	const layout = "2006-01-02T15:04:05"
	loc, _ := time.LoadLocation("Local")

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
	}

	for _, v := range p.Stand.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}
func (p CarouselSlot) getResourceID() (name string, from time.Time, to time.Time) {

	const layout = "2006-01-02T15:04:05"
	loc, _ := time.LoadLocation("Local")

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
	}

	for _, v := range p.Carousel.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}

func (p ChuteSlot) getResourceID() (name string, from time.Time, to time.Time) {

	const layout = "2006-01-02T15:04:05"
	loc, _ := time.LoadLocation("Local")

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
	}

	for _, v := range p.Chute.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}

func (p GateSlot) getResourceID() (name string, from time.Time, to time.Time) {

	const layout = "2006-01-02T15:04:05"
	loc, _ := time.LoadLocation("Local")

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(layout, v.Text, loc)
			continue
		}
	}

	for _, v := range p.Gate.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}

type CheckInSlot struct {
	Value   []Value `xml:"Value"`
	CheckIn CheckIn `xml:"CheckIn"`
}
type CheckInSlots struct {
	CheckInSlot []CheckInSlot `xml:"CheckInSlot" json:"CheckInSlot,omitempty"`
}
type Chute struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}
type ChuteSlot struct {
	Value []Value `xml:"Values"`
	Chute Chute   `xml:"Chute"`
}
type ChuteSlots struct {
	ChuteSlot []ChuteSlot `xml:"ChuteSlot" json:"ChuteSlot,omitempty"`
}
