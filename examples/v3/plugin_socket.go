// +build consumer

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pact-foundation/pact-go/utils"
)

const (
	host      = "localhost"
	port      = 3333
	adminPort = 4444
	network   = "tcp"
)

func main() {
	startProvider()
}

func startTCPListener(s *session) {
	l, err := net.Listen(network, fmt.Sprintf("%s:%d", host, s.port))
	if err != nil {
		log.Println("[DEBUG] Error listening:", err.Error())
		os.Exit(1)
	}

	defer l.Close()
	log.Println("[DEBUG] Listening on host:", host, "port:", s.port)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("[DEBUG] Error accepting: ", err.Error())
			os.Exit(1)
		}

		go handleRequest(conn, s)
	}
}

// Handles TCP requests.
func handleRequest(conn net.Conn, s *session) {
	defer conn.Close()

	buf := make([]byte, 1024) // TODO: crude buffer
	l, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		log.Println("[DEBUG] Error reading:", err.Error())
		conn.Write([]byte(""))
	} else {
		response := matchRequest(strings.TrimSpace(string(buf[:l])), s)
		conn.Write([]byte(response))
	}
}

func matchRequest(message string, s *session) string {
	log.Println(fmt.Sprintf("[DEBUG] matching request: '%s', with session '%v'", message, s))

	if response, ok := s.interactions[message]; ok {
		log.Println("[DEBUG] found match!", response)
		s.matchedInteractions = append(s.matchedInteractions, message)
		return response.Response
	}

	return ""
}

func sessionMismatches(s *session) mismatched {
	mismatchedInteractions := make(interactions)
	res := mismatched{
		Mismatches: make([]mismatchDetail, 0),
	}

	// copy new map to preserve
	for k, v := range s.interactions {
		mismatchedInteractions[k] = v
	}

	for _, match := range s.matchedInteractions {
		delete(mismatchedInteractions, match)
	}

	for _, unmatched := range mismatchedInteractions {
		res.Mismatches = append(res.Mismatches, mismatchDetail{
			Actual:   "",
			Expected: unmatched.Message,
			Mismatch: fmt.Sprintf("expected message '%s', but got none", unmatched.Message),
		})
	}

	return res
}

type interaction struct {
	Message   string `json:"message"`   // consumer request
	Response  string `json:"response"`  // expected response
	Delimeter string `json:"delimeter"` // how to determine message boundary
}

type interactions map[string]interaction

var sessions map[string]*session

// Plugin Bits
type session struct {
	id                  string       // UUID (supplied by the Pact framework)
	interactions        interactions // Key is the TCP message, val is the requested response
	matchedInteractions []string     // Store the matched requests for the session
	port                int          // Port the session is running on TODO: this probably ought to be more general than TCP (e.g. socket addresses too?)
}

type mismatchDetail struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
	Mismatch string `json:"mismatch"`
}

// mismatchedRequest contains details of any request mismatches during pact verification
type mismatched struct {
	Mismatches []mismatchDetail `json:"mismatches"`
}

// Starts the plugin API
func startProvider() {
	// Initialise a sessions store
	sessions = make(map[string]*session)

	// Start a channel to receive TCP session creations
	tcpSessions := make(chan *session)

	// Create new TCP listeners on demand
	go listenForSessions(tcpSessions)

	// Create the TCP Plugin admin server
	router := gin.Default()
	router.POST("/sessions", createSession(tcpSessions))
	router.POST("/sessions/:id/interactions", loadInteractions)
	router.GET("/sessions/:id/mismatches", mismatches)

	router.Run(fmt.Sprintf(":%d", adminPort))
}

type sessionRequest struct {
	// TODO
}

type sessionResponse struct {
	ID        string `json:"id"`
	Port      int    `json:"port"`      // Port for the client to communicate with, should be dynamic per session
	AdminPort int    `json:"adminPort"` // Port for the framework to communicate with, should be dynamic per session
}

type interactionsRequest struct {
	Interactions []interaction `json:"interactions"`
}

// Reads from channel, creates a new TCP session
func listenForSessions(c chan *session) {
	log.Println("[DEBUG] starting the session creator")
	for i := range c {
		log.Println("[DEBUG] starting a new TCP session", i)
		go startTCPListener(i)
	}
}

// POST /sessions
func createSession(tcpSessions chan *session) func(*gin.Context) {
	return func(c *gin.Context) {
		var json sessionRequest

		if c.BindJSON(&json) == nil {
			log.Println("[DEBUG] starting new session", json)
			port, _ := utils.GetFreePort()

			id := uuid.New().String()
			session := &session{
				id:   id,
				port: port,
			}
			response := sessionResponse{
				Port: port,
				ID:   id,
			}
			sessions[id] = session

			// send to channel to create a new session, and start a TCP server
			tcpSessions <- session

			c.JSON(http.StatusOK, response)
		}
	}
}

// POST /sessions/:id/interactions
func loadInteractions(c *gin.Context) {
	id := c.Param("id")

	if session, ok := sessions[id]; ok {
		var json interactionsRequest

		if c.BindJSON(&json) == nil {
			log.Println("[DEBUG] loading interactions for session", session, json)

			session.interactions = make(interactions, len(json.Interactions))
			for _, v := range json.Interactions {
				session.interactions[v.Message] = v
			}
			log.Println("[DEBUG] loaded interactions for session", session)

			c.JSON(http.StatusOK, nil)
		}

	} else {
		c.JSON(http.StatusNotFound, nil)
	}

}

// GET /sessions/:id/mismatches
func mismatches(c *gin.Context) {
	id := c.Param("id")

	if session, ok := sessions[id]; ok {
		log.Println("[DEBUG] finding mismatches for session", session)

		res := sessionMismatches(session)
		c.JSON(http.StatusOK, res)
	} else {
		log.Println("[DEBUG] unable to find session with id", id)
		c.JSON(http.StatusNotFound, nil)
	}

}
