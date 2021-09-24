package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type Master struct {
	L    *log.Logger
	// Slice of all clients, waiting their pair.
	Asks []*AskReq
}

type AskReq struct {
	// Id is used to find a pair with the same Id.
	Id                string `json:"id"`
	CommunicationPort string `json:"communication_port"`
	AssistancePort    string `json:"assistance_port"`

	// Used to leave the HandleAsk method unfinished.
	// This will allow us to use rw later.
	wg          sync.WaitGroup      `json:"-"`
	rw          http.ResponseWriter `json:"-"`
	// Time after which the request will be removed from the list.
	TimeoutTime time.Time           `json:"-"`
	Adr         string              `json:"-"`
}

func NewMaster(logger *log.Logger) *Master {
	var m Master
	m.L = logger
	m.Asks = make([]*AskReq, 0)
	return &m
}

func (m *Master) HandleAsk(rw http.ResponseWriter, r *http.Request) {
	m.L.Println("Got request from", r.Host)
	// Write request to new AskReq object.
	a, err := m.unpackAskRequest(rw, r)
	if err != nil {
		m.SendError(rw, "Can't read request body.", http.StatusBadRequest)
	}

	// Try to find pair in saved asks.
	m.cycleAsks(a)
	// Wait until pair will be found or timeout happens.
	a.wg.Wait()
}

// removeOldAsks removes all requests that timed out earlier than the current time.
func (m *Master) removeOldAsks(){
	var skip = false
	for i := range m.Asks {
		if skip {
			i--
			skip = false
		}
		if m.Asks[i].TimeoutTime.Before(time.Now()) {
			m.Asks[i].wg.Done()
			m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			skip = true
		}
	}
}

// cycleAsks checks the list of pending clients for a match id.
func (m *Master) cycleAsks(a *AskReq) {
	var skip, added = false, false
	for i := range m.Asks {
		if skip {
			i--
			skip = false
		}

		if m.Asks[i].Id == a.Id {
			if m.Asks[i].Adr == a.Adr {
				added = true
				continue
			}

			a.wg.Add(1)
			err := m.pairClients(m.Asks[i], a)
			if err != nil {
				log.Println(err)
			}

			log.Println(m.Asks)
			m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			log.Println(m.Asks)
			skip = true
			added = true
		}
	}

	if !added {
		m.addAsk(a)
	}

	for i := range m.Asks {
		log.Println(*m.Asks[i])
		log.Println(m.Asks[i].Adr)
	}
}

func (m *Master) addAsk(a *AskReq) {
	//TODO: Add timeout config.
	a.wg.Add(1)
	a.TimeoutTime = time.Now().Add(time.Second * 30)
	m.Asks = append(m.Asks, a)
}

func (m *Master) pairClients(c1, c2 *AskReq) (err error) {
	err = m.sendResponse(c1, c2)
	if err != nil {
		return err
	}
	err = m.sendResponse(c2, c1)
	if err != nil {
		return err
	}

	return nil
}

func (m *Master) sendResponse(c1, c2 *AskReq) error {
	c1.rw.WriteHeader(http.StatusOK)
	n, err := c1.rw.Write([]byte(fmt.Sprintf("{\"ip\":\"%s\",\"communication_port\":\"%s\", \"assistance_port\":\"%s\"}", c2.Adr, c2.CommunicationPort, c2.AssistancePort)))
	log.Println(n)
	if err != nil {
		return err
	}

	c1.wg.Done()
	log.Println("Response written.")

	return nil
}

func (m *Master) unpackAskRequest(rw http.ResponseWriter, r *http.Request) (*AskReq, error) {
	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	m.L.Println("Body: ", string(reqBytes))

	a := AskReq{}

	err = json.Unmarshal(reqBytes, &a)
	if err != nil {
		return nil, err
	}

	a.Adr = r.RemoteAddr
	a.rw = rw

	return &a, nil
}

func (m *Master) SendError(rw http.ResponseWriter, msg string, code int) {
	rw.WriteHeader(code)
	rw.Write([]byte("{\"error\": \"" + msg + "\"}"))
	m.L.Println("[ERROR] " + msg)
}
