package main

import (
	"time"
)

type ParameterValuePair struct {
	Parameter string `json:"Parameter,omitempty"`
	Value     string `json:"Value,omitempty"`
}

type UserProfile struct {
	UserName                      string               `json:"username"`
	Key                           string               `json:"key"`
	AllowedAirports               []string             `json:"allowedairports"`
	AllowedAirlines               []string             `json:"allowedairlines"`
	AllowedCustomFields           []string             `json:"allowedcustomfields"`
	QueryableCustomFields         []string             `json:"queryablecustomfields"`
	DefaultAirport                string               `json:"defaultairport"`
	OverrideAirport               string               `json:"overrideairport"`
	DefaultAirline                string               `json:"defaultairline"`
	OverrideAirline               string               `json:"overrideairline"`
	DefaultQueryableCustomFields  []ParameterValuePair `json:"defaultqueryablecustomfields"`
	OverrideQueryableCustomFields []ParameterValuePair `json:"overridequeryablecustomfields"`
}

type Users struct {
	Users []UserProfile `json:"users"`
}

type Repository struct {
	Airport           string `json:"airport"`
	URL               string `json:"url"`
	Token             string `json:"token"`
	WindowMin         int    `json:"windowminimum"`
	WindowMax         int    `json:"windowmaximum"`
	ListenerType      string `json:"listenertype"`
	ListenerQueue     string `json:"listenerqueue"`
	ChunkSize         int    `json:"chunksize"`
	Flights           map[string]Flight
	CurrentLowerLimit time.Time
	CurrentUpperLimit time.Time
}

type Repositories struct {
	Repositories []Repository `json:"airports"`
}
type Response struct {
	User             string               `json:"User,omitempty"`
	AirportCode      string               `json:"AirportCode,omitempty"`
	From             string               `json:"FlightsFrom,omitempty"`
	To               string               `json:"FlightsTo,omitempty"`
	NumberOfFlights  int                  `json:"NumberOfFlights,omitempty"`
	Direction        string               `json:"Direction,omitempty"`
	CustomFieldQuery []ParameterValuePair `json:"CustomFieldQueries,omitempty"`
	Warnings         []string             `json:"Warnings,omitempty"`
	Errors           []string             `json:"Errors,omitempty"`
	Flights          []Flight             `json:"Flights,omitempty"`
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
	UserProfile                UserProfile
	PresentQueryableParameters []ParameterValuePair
}
