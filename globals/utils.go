package globals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flightresourcerestapi/models"
)

func Contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
func CleanJSON(sb strings.Builder) string {

	s := sb.String()
	if last := len(s) - 1; last >= 0 && s[last] == ',' {
		s = s[:last]
	}

	s = s + "}"

	return s
}

func GetUserProfiles() []models.UserProfile {

	var users models.Users
	if err := UserViper.Unmarshal(&users); err != nil {
		return nil
	}

	return users.Users
}
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func ExePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func ExeTime(name string) func() {
	start := time.Now()
	return func() {
		MetricsLogger.Info(fmt.Sprintf("%s execution time: %v", name, time.Since(start)))
	}
}

// func (ll *models.ResourceLinkedList) AddNodes(nodes []FixedResource) {
// 	for _, node := range nodes {
// 		newNode := ResourceAllocationStruct{Resource: node}
// 		ll.AddNode(newNode)
// 	}
// }

// // AddNode adds a new node to the end of the doubly linked list.
// func (ll *ResourceLinkedList) AddNode(newNode ResourceAllocationStruct) {

// 	newNode.PrevNode = ll.Tail
// 	newNode.NextNode = nil

// 	if ll.Tail != nil {
// 		ll.Tail.NextNode = &newNode
// 	}

// 	ll.Tail = &newNode

// 	if ll.Head == nil {
// 		ll.Head = &newNode
// 	}
// }

// func (ll *ResourceLinkedList) RemoveNode(removeNode ResourceAllocationStruct) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.Resource.Name == removeNode.Resource.Name {
// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = currentNode.NextNode
// 			} else {
// 				ll.Head = currentNode.NextNode
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = currentNode.PrevNode
// 			} else {
// 				ll.Tail = currentNode.PrevNode
// 			}
// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			return // Node found and removed, exit the function
// 		}

// 		currentNode = currentNode.NextNode
// 	}
// }

// func (r *Repository) RemoveFlightAllocation(flightID string) {
// 	r.CheckInList.RemoveFlightAllocation(flightID)
// 	r.GateList.RemoveFlightAllocation(flightID)
// 	r.StandList.RemoveFlightAllocation(flightID)
// 	r.CarouselList.RemoveFlightAllocation(flightID)
// 	r.ChuteList.RemoveFlightAllocation(flightID)
// }

// func (ll *ResourceLinkedList) RemoveFlightAllocation(flightID string) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		currentNode.FlightAllocationsList.RemoveFlightAllocations(flightID)
// 		currentNode = currentNode.NextNode
// 	}
// }

// func (ll *ResourceLinkedList) NumberOfFlightAllocations() (n int) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		n = n + currentNode.FlightAllocationsList.Len()
// 		currentNode = currentNode.NextNode
// 	}
// 	return
// }

// func (ll *ResourceLinkedList) AddAllocation(node AllocationItem) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.Resource.Name == node.ResourceID {
// 			currentNode.FlightAllocationsList.AddNode(node)
// 			break
// 		}
// 		currentNode = currentNode.NextNode
// 	}
// }

// // FlightLinkedList represents the doubly linked list.
// type FlightLinkedList struct {
// 	Head *Flight
// 	Tail *Flight
// }

// AddNode adds a new node to the end of the doubly linked list.
// func (ll *FlightLinkedList) AddNode(newNode Flight) {

// 	newNode.PrevNode = ll.Tail
// 	newNode.NextNode = nil

// 	if ll.Tail != nil {
// 		ll.Tail.NextNode = &newNode
// 	}

// 	ll.Tail = &newNode

// 	if ll.Head == nil {
// 		ll.Head = &newNode
// 	}
// }

// func (ll *FlightLinkedList) RemoveNode(removeNode Flight) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.GetFlightID() == removeNode.GetFlightID() {
// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = currentNode.NextNode
// 			} else {
// 				ll.Head = currentNode.NextNode
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = currentNode.PrevNode
// 			} else {
// 				ll.Tail = currentNode.PrevNode
// 			}

// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			return // Node found and removed, exit the function
// 		}

// 		currentNode = currentNode.NextNode
// 	}
// }

// func (ll *FlightLinkedList) RemoveExpiredNode(from time.Time) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.GetSDO().Before(from) {
// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = currentNode.NextNode
// 			} else {
// 				ll.Head = currentNode.NextNode
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = currentNode.PrevNode
// 			} else {
// 				ll.Tail = currentNode.PrevNode
// 			}

// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			return // Node found and removed, exit the function
// 		}

// 		currentNode = currentNode.NextNode
// 	}
// }
// func (ll *FlightLinkedList) ReplaceOrAddNode(node Flight) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.GetFlightID() == node.GetFlightID() {
// 			// Replace the entire node
// 			node.PrevNode = currentNode.PrevNode
// 			node.NextNode = currentNode.NextNode

// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = &node
// 			} else {
// 				ll.Head = &node
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = &node
// 			} else {
// 				ll.Tail = &node
// 			}

// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			return // Node found and replaced, exit the function
// 		}
// 		currentNode = currentNode.NextNode
// 	}

// 	// If the node is not found, add it to the end of the linked list
// 	ll.AddNode(node)
// }

// func (ll *FlightLinkedList) Len() int {
// 	currentNode := ll.Head
// 	count := 0

// 	for currentNode != nil {
// 		count++
// 		currentNode = currentNode.NextNode
// 	}

// 	return count
// }

// func (ll *ResourceLinkedList) Len() int {
// 	currentNode := ll.Head
// 	count := 0

// 	for currentNode != nil {
// 		count++
// 		currentNode = currentNode.NextNode
// 	}

// 	return count
// }

// type AllocationLinkedList struct {
// 	Head *AllocationItem
// 	Tail *AllocationItem
// }

// func (ll *AllocationLinkedList) Len() int {
// 	currentNode := ll.Head
// 	count := 0

// 	for currentNode != nil {
// 		count++
// 		currentNode = currentNode.NextNode
// 	}

// 	return count
// }

// func (ll *AllocationLinkedList) RemoveExpiredNode(from time.Time) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.From.Before(from) {
// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = currentNode.NextNode
// 			} else {
// 				ll.Head = currentNode.NextNode
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = currentNode.PrevNode
// 			} else {
// 				ll.Tail = currentNode.PrevNode
// 			}

// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			return // Node found and removed, exit the function
// 		}

// 		currentNode = currentNode.NextNode
// 	}
// }
// func (ll *AllocationLinkedList) RemoveFlightAllocations(flightID string) {
// 	currentNode := ll.Head

// 	for currentNode != nil {
// 		if currentNode.FlightID == flightID {
// 			mapMutex.Lock()
// 			if currentNode.PrevNode != nil {
// 				currentNode.PrevNode.NextNode = currentNode.NextNode
// 			} else {
// 				ll.Head = currentNode.NextNode
// 			}

// 			if currentNode.NextNode != nil {
// 				currentNode.NextNode.PrevNode = currentNode.PrevNode
// 			} else {
// 				ll.Tail = currentNode.PrevNode
// 			}

// 			currentNode.PrevNode = nil
// 			currentNode.NextNode = nil

// 			mapMutex.Unlock()
// 			//return // Node found and removed, exit the function
// 		}

// 		currentNode = currentNode.NextNode
// 	}
// }
// func (ll *AllocationLinkedList) AddNode(newNode AllocationItem) {

// 	newNode.PrevNode = ll.Tail
// 	newNode.NextNode = nil

// 	if ll.Tail != nil {
// 		ll.Tail.NextNode = &newNode
// 	}

// 	ll.Tail = &newNode

// 	if ll.Head == nil {
// 		ll.Head = &newNode
// 	}
// }
