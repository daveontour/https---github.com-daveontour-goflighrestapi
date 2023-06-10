package main

import (
	"time"
)

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
