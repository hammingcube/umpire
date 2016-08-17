package main

import (
	"fmt"
	"sync"
	"time"
)

type TicketStatus int

const (
	Unknown TicketStatus = iota
	Pending
	Done
)

func (ts TicketStatus) String() string {
	switch ts {
	case Pending:
		return "Pending"
	case Done:
		return "Done"
	default:
		return "Unknown"
	}
}

type Ticket struct {
	Key    string       `json:"key"`
	Status TicketStatus `json:"status"`
}

type ErrNotFound struct{}

func (e ErrNotFound) Error() string {
	return "Not Found"
}

type TicketStore struct {
	store map[string]*Ticket
	*sync.Mutex
}

func NewTicketStore() *TicketStore {
	return &TicketStore{make(map[string]*Ticket), &sync.Mutex{}}
}

func (tickets *TicketStore) add(t *Ticket) {
	tickets.Lock()
	defer tickets.Unlock()
	tickets.store[t.Key] = t
}

func (tickets *TicketStore) status(key string) TicketStatus {
	tickets.Lock()
	defer tickets.Unlock()
	t, ok := tickets.store[key]
	if !ok {
		return Unknown
	}
	return t.Status
}

func (tickets *TicketStore) update(key string, state TicketStatus) {
	tickets.Lock()
	defer tickets.Unlock()
	t, _ := tickets.store[key]
	if t != nil {
		t.Status = state
	}
}

func (tickets *TicketStore) remove(key string) {
	tickets.Lock()
	defer tickets.Unlock()
	delete(tickets.store, key)
}

var tickets = NewTicketStore()

func doWork() {
	time.Sleep(2 * time.Second)
}

func handleDoWork() string {
	key := "hello"
	ticket := &Ticket{key, Pending}
	tickets.add(ticket)
	go func() {
		defer func() { tickets.update(key, Done) }()
		doWork()
	}()
	return key
}

func main() {
	key := handleDoWork()
	fmt.Println(tickets.status(key))
	time.Sleep(3 * time.Second)
	fmt.Println(tickets.status(key))
}
