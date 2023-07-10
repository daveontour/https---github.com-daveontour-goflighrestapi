package main

import (
	//	"encoding/json"
	//	"encoding/xml"

	"fmt"
	"io/ioutil"

	"strings"

	//	"log"
	"net/http"
	"os"

	//	"path/filepath"
	"time"
)

type FixedResource struct {
	ID               string `xml:"Id"`
	ResourceTypeCode string `xml:"ResourceTypeCode"`
	Name             string `xml:"Name"`
	Area             string `xml:"Area"`
}
type ArrayOfFixedResource struct {
	FixedResources []FixedResource `xml:"FixedResource"`
}

type ServiceConfig struct {
	ServiceName        string `json:"servicename"`
	ServicDisplayName  string `json:"servicedisplayname"`
	ServiceDescription string `json:"servicedescription"`
	ServiceIPPort      string `json:"serviceipport"`
}

type UserProfile struct {
	UserName              string   `json:"username"`
	Key                   string   `json:"key"`
	AllowedAirports       []string `json:"allowedairports"`
	AllowedAirlines       []string `json:"allowedairlines"`
	AllowedCustomFields   []string `json:"allowedcustomfields"`
	QueryableCustomFields []string `json:"queryablecustomfields"`
	DefaultAirport        string   `json:"defaultairport"`
	OverrideAirport       string   `json:"overrideairport"`
	DefaultAirline        string   `json:"defaultairline"`
	OverrideAirline       string   `json:"overrideairline"`
	//	DefaultQueryableCustomFields  []ParameterValuePair `json:"defaultqueryablecustomfields"`
	//	OverrideQueryableCustomFields []ParameterValuePair `json:"overridequeryablecustomfields"`
}

type Users struct {
	Users []UserProfile `json:"users"`
}

type Repository struct {
	Airport           string `json:"airport"`
	URL               string `json:"url"`
	RestURL           string `json:"resturl"`
	Token             string `json:"token"`
	WindowMin         int    `json:"windowminimum"`
	WindowMax         int    `json:"windowmaximum"`
	ListenerType      string `json:"listenertype"`
	ListenerQueue     string `json:"listenerqueue"`
	ChunkSize         int    `json:"chunksize"`
	CurrentLowerLimit time.Time
	CurrentUpperLimit time.Time
}

type Repositories struct {
	Repositories []Repository `json:"airports"`
}
type Response struct {
	User            string `json:"User,omitempty"`
	AirportCode     string `json:"AirportCode,omitempty"`
	From            string `json:"FlightsFrom,omitempty"`
	To              string `json:"FlightsTo,omitempty"`
	NumberOfFlights int    `json:"NumberOfFlights,omitempty"`
	Direction       string `json:"Direction,omitempty"`
	//	CustomFieldQuery []ParameterValuePair `json:"CustomFieldQueries,omitempty"`
	Warnings []string `json:"Warnings,omitempty"`
	Errors   []string `json:"Errors,omitempty"`
}

func (r *Response) AddWarning(w string) {
	r.Warnings = append(r.Warnings, w)
}
func (r *Response) AddError(w string) {
	r.Errors = append(r.Errors, w)
}

type Request struct {
	Direction   string
	Airline     string
	From        string
	To          string
	UserProfile UserProfile
	//	PresentQueryableParameters []ParameterValuePair
}

type Stand struct {
	Id string
}

type Gate struct {
}

type Checkin struct {
}

type Chute struct {
}

type Carousel struct {
}

type StandAllocation struct {
	Stand Stand
	From  time.Time
	To    time.Time
}

type Allocation struct {
	start    time.Time
	end      time.Time
	flightId string
}

type AllocationLinkItem struct {
	next       *AllocationLinkItem
	prev       *AllocationLinkItem
	allocation Allocation
}

type AllocationList struct {
	Resoure FixedResource
	head    *AllocationLinkItem
	tail    *AllocationLinkItem
}

type AllocationMap struct {
	allocationMap map[string]AllocationList
}

func (list *AllocationList) insert(startdata, enddata time.Time, flightId string) {

	newAllocatioNode := &Allocation{
		start:    startdata,
		end:      enddata,
		flightId: flightId,
	}

	newNode := &AllocationLinkItem{
		allocation: *newAllocatioNode,
		prev:       nil,
		next:       nil,
	}

	if list.head == nil {
		list.head = newNode
		list.tail = newNode
	} else if startdata.Before(list.head.allocation.start) || startdata == list.head.allocation.start {
		newNode.next = list.head
		list.head.prev = newNode
		list.head = newNode
	} else if startdata.After(list.tail.allocation.start) {
		newNode.prev = list.tail
		list.tail.next = newNode
		list.tail = newNode
	} else {
		currentNode := list.head.next
		for currentNode != nil {
			if startdata.Before(currentNode.allocation.start) || startdata == currentNode.allocation.start {
				newNode.prev = currentNode.prev
				newNode.next = currentNode
				currentNode.prev.next = newNode
				currentNode.prev = newNode
				break
			}
			currentNode = currentNode.next
		}
	}
}

func (list *AllocationList) remove(flightId string) {
	currentNode := list.head
	for currentNode != nil {
		if currentNode.allocation.flightId == flightId {
			if currentNode == list.head {
				list.head = currentNode.next
				if list.head != nil {
					list.head.prev = nil
				} else {
					list.tail = nil
				}
			} else if currentNode == list.tail {
				list.tail = currentNode.prev
				list.tail.next = nil
			} else {
				currentNode.prev.next = currentNode.next
				currentNode.next.prev = currentNode.prev
			}
			break
		}
		currentNode = currentNode.next
	}
}

func (m *AllocationMap) FindAllocationsBetween(from, to time.Time) []Allocation {

	allocations := []Allocation{}
	for _, l := range m.allocationMap {
		p := l.head

		for p.next != nil {
			if (p.allocation.start.After(from) || p.allocation.start == from) && (p.allocation.start.Before(to) || p.allocation.start == to) {
				allocations = append(allocations, p.allocation)
				p = p.next
				continue
			}

			if (p.allocation.end.After(from) || p.allocation.end == from) && (p.allocation.end.Before(to) || p.allocation.end == to) {
				allocations = append(allocations, p.allocation)
				p = p.next
				continue
			}
			p = p.next
		}
	}

	return allocations
}

func (m *AllocationMap) GetCurrentAllocations() []Allocation {
	return m.GetAllocationsAtTime(time.Now())
}

func (m *AllocationMap) GetAllocationsAtTime(pointInTime time.Time) []Allocation {

	allocations := []Allocation{}

	for _, l := range m.allocationMap {
		p := l.head
		for p.next != nil {
			if (p.allocation.start.Before(pointInTime) || p.allocation.start == pointInTime) && (p.allocation.end.After(pointInTime) || p.allocation.end == pointInTime) {
				allocations = append(allocations, p.allocation)
			}
			p = p.next
		}
	}

	return allocations
}

func (m *AllocationMap) FindAllocationsForResource(from, to time.Time, resourceId string) []Allocation {

	allocations := []Allocation{}

	l, found := m.allocationMap[resourceId]

	if !found {
		return allocations
	}

	p := l.head

	for p.next != nil {
		if (p.allocation.start.After(from) || p.allocation.start == from) && (p.allocation.start.Before(to) || p.allocation.start == to) && strings.Contains(p.allocation.flightId, resourceId) {
			allocations = append(allocations, p.allocation)
			p = p.next
			continue
		}

		if (p.allocation.end.After(from) || p.allocation.end == from) && (p.allocation.end.Before(to) || p.allocation.end == to) && strings.Contains(p.allocation.flightId, resourceId) {
			allocations = append(allocations, p.allocation)
			p = p.next
			continue
		}
		p = p.next
	}

	return allocations
}

func (m *AllocationMap) FindAllocationsForFlight(from, to time.Time, flightId string) []Allocation {

	allocations := []Allocation{}

	for _, l := range m.allocationMap {
		p := l.head

		for p.next != nil {
			if (p.allocation.start.After(from) || p.allocation.start == from) && (p.allocation.start.Before(to) || p.allocation.start == to) && strings.Contains(p.allocation.flightId, flightId) {
				allocations = append(allocations, p.allocation)
				p = p.next
				continue
			}

			if (p.allocation.end.After(from) || p.allocation.end == from) && (p.allocation.end.Before(to) || p.allocation.end == to) && strings.Contains(p.allocation.flightId, flightId) {
				allocations = append(allocations, p.allocation)
				p = p.next
				continue
			}
			p = p.next
		}
	}

	return allocations
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
