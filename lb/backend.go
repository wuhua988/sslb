package lb

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	errInvalidBackend = errors.New("Invalid backend")
)

//TODO: Need to rebalance the score when backend back to active

// Backend structure
type Backend struct {
	Mutex sync.Mutex

	Name      string
	Address   string
	Heartbeat string

	// Consider inactive after max inactiveAfter
	InactiveAfter int

	HeartbeatTime time.Duration // Heartbeat time if health
	RetryTime     time.Duration // Retry to time after failed

	// The last request failed
	Failed bool
	Active bool
	Tries  int
	Score  int
}

type Backends []*Backend

type ByScore []*Backend

func (a ByScore) Len() int           { return len(a) }
func (a ByScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByScore) Less(i, j int) bool { return a[i].Score < a[j].Score }

func NewBackend(name string, address string, heartbeat string,
	inactiveAfter, heartbeatTime, retryTime int) *Backend {
	return &Backend{
		Name:      name,
		Address:   address,
		Heartbeat: address,

		InactiveAfter: inactiveAfter,
		HeartbeatTime: time.Duration(heartbeatTime) * time.Millisecond,
		RetryTime:     time.Duration(retryTime) * time.Millisecond,

		Failed: true,
		Active: true,
		Tries:  0,
		Score:  0,
	}
}

// Monitoring the backend, can add or remove if heartbeat fail
func (b *Backend) HeartCheck() {
	go func() {
		for {
			resp, err := http.Head(b.Heartbeat)
			if err != nil {
				// Max tries before consider inactive
				if b.Tries >= b.InactiveAfter {
					log.Printf("Backend inactive [%s]", b.Name)
					b.Mutex.Lock()
					b.Active = false
					b.Mutex.Unlock()
				} else {
					// Ok that guy it's out of the game
					b.Mutex.Lock()
					b.Failed = true
					b.Tries += 1
					b.Mutex.Unlock()
					log.Printf("Error to check address [%s] name [%s] tries [%d]", b.Heartbeat, b.Name, b.Tries)
				}
			} else {
				defer resp.Body.Close()

				if b.Failed {
					log.Printf("Backend active [%s]", b.Name)
				}

				// Ok, let's keep working boys
				b.Mutex.Lock()
				b.Failed = false
				b.Active = true
				b.Tries = 0
				b.Mutex.Unlock()
			}

			if b.Failed {
				time.Sleep(b.RetryTime)
			} else {
				time.Sleep(b.HeartbeatTime)
			}
		}
	}()
}
