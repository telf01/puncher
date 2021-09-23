package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type Master struct {
	L    *log.Logger
	Asks []*AskReq
}

type AskReq struct {
	Id                string `json:"id"`
	CommunicationPort string `json:"communication_port"`
	AssistancePort    string `json:"assistance_port"`

	rw          http.ResponseWriter `json:"-"`
	TimeoutTime time.Time           `json:"-"`
	Adr         net.IP              `json:"-"`
}

func NewMaster(logger *log.Logger) *Master {
	var m Master
	m.L = logger
	m.Asks = make([]*AskReq, 0)
	return &m
}

func (m *Master) HandleAsk(rw http.ResponseWriter, r *http.Request) {
	m.L.Println("Got request from", r.Host)
	a, err := m.unpackAskRequest(rw, r)
	if err != nil {
		m.SendError(rw, "Can't read request body.", http.StatusBadRequest)
	}

	m.cycleAsks(a)
}

func (m *Master) cycleAsks(a *AskReq) {
	var skip, added = false, false
	for i := range m.Asks {
		if skip {
			i--
			skip = false
		}
		if m.Asks[i].TimeoutTime.After(time.Now()) {
			m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			skip = true
		}

		if m.Asks[i].Id == a.Id {
			if string(m.Asks[i].Adr) == string(a.Adr) {
				added = true
				continue
			}

			m.pairClients(m.Asks[i], a)
			m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			skip = true
			added = true
		}
	}

	if !added {
		m.addAsk(a)
	}

	for i := range m.Asks {
		log.Println(*m.Asks[i])
	}
}

func (m *Master) addAsk(a *AskReq) {
	//TODO: Add timeout config.
	a.TimeoutTime = time.Now().Add(time.Second * 30)
	m.Asks = append(m.Asks, a)
}

func (m *Master) pairClients(c1, c2 *AskReq) {
	m.sendResponse(c1, c2)
	m.sendResponse(c2, c1)
}

func (m *Master) sendResponse(c1, c2 *AskReq) {
	c1.rw.WriteHeader(http.StatusOK)
	c1.rw.Header().Add("Content-Type", "application/json")
	c1.rw.Write([]byte(fmt.Sprintf("{\"ip\":\"%s\",\"communication_port\":\"%s\", \"assistance_port\":\"%s\"}", c2.Adr.String(), c2.CommunicationPort, c2.AssistancePort)))
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

	a.rw = rw

	return &a, nil
}

func (m *Master) SendError(rw http.ResponseWriter, msg string, code int) {
	rw.WriteHeader(code)
	rw.Write([]byte("{\"error\": \"" + msg + "\"}"))
	m.L.Println("[ERROR] " + msg)
}
