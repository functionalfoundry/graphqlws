package graphqlws

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	// Constants for operation message types
	gqlConnectionInit      = "connection_init"
	gqlConnectionAck       = "connection_ack"
	gqlConnectionKeepAlive = "ka"
	gqlConnectionError     = "connection_error"
	gqlConnectionTerminate = "connection_terminate"
	gqlStart               = "start"
	gqlData                = "data"
	gqlError               = "error"
	gqlComplete            = "complete"
	gqlStop                = "stop"

	// Maximum size of incoming messages
	readLimit = 4096

	// Timeout for outgoing messages
	writeTimeout = 10 * time.Second
)

// InitMessagePayload defines the parameters of a connection
// init message.
type InitMessagePayload struct {
	AuthToken string `json:"authToken"`
}

// StartMessagePayload defines the parameters of an operation that
// a client requests to be started.
type StartMessagePayload struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operationName"`
}

// DataMessagePayload defines the result data of an operation.
type DataMessagePayload struct {
	Data   interface{} `json:"data"`
	Errors []error     `json:"errors"`
}

// OperationMessage represents a GraphQL WebSocket message.
type OperationMessage struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (msg OperationMessage) String() string {
	s, _ := json.Marshal(msg)
	if s != nil {
		return string(s)
	}
	return "<invalid>"
}

// AuthenticateFunc is a function that resolves an auth token
// into a user (or returns an error if that isn't possible).
type AuthenticateFunc func(token string) (interface{}, error)

// ConnectionEventHandlers define the event handlers for a connection.
// Event handlers allow other system components to react to events such
// as the connection closing or an operation being started or stopped.
type ConnectionEventHandlers struct {
	// Close is called whenever the connection is closed, regardless of
	// whether this happens because of an error or a deliberate termination
	// by the client.
	Close func(Connection)

	// StartOperation is called whenever the client demands that a GraphQL
	// operation be started (typically a subscription). Event handlers
	// are expected to take the necessary steps to register the operation
	// and send data back to the client with the results eventually.
	StartOperation func(Connection, string, *StartMessagePayload) []error

	// StopOperation is called whenever the client stops a previously
	// started GraphQL operation (typically a subscription). Event handlers
	// are expected to unregister the operation and stop sending result
	// data to the client.
	StopOperation func(Connection, string)
}

// ConnectionConfig defines the configuration parameters of a
// GraphQL WebSocket connection.
type ConnectionConfig struct {
	Authenticate  AuthenticateFunc
	EventHandlers ConnectionEventHandlers
}

// Connection is an interface to represent GraphQL WebSocket connections.
// Each connection is associated with an ID that is unique to the server.
type Connection interface {
	// ID returns the unique ID of the connection.
	ID() string

	// User returns the user associated with the connection (or nil).
	User() interface{}

	// SendData sends results of executing an operation (typically a
	// subscription) to the client.
	SendData(string, *DataMessagePayload)

	// SendError sends an error to the client.
	SendError(error)
}

/**
 * The default implementation of the Connection interface.
 */

type connection struct {
	id         string
	ws         *websocket.Conn
	config     ConnectionConfig
	logger     *log.Entry
	outgoing   chan OperationMessage
	user       interface{}
	closeMutex *sync.Mutex
	closed     bool
}

func operationMessageForType(messageType string) OperationMessage {
	return OperationMessage{
		Type: messageType,
	}
}

// NewConnection establishes a GraphQL WebSocket connection. It implements
// the GraphQL WebSocket protocol by managing its internal state and handling
// the client-server communication.
func NewConnection(ws *websocket.Conn, config ConnectionConfig) Connection {
	conn := new(connection)
	conn.id = uuid.New().String()
	conn.ws = ws
	conn.config = config
	conn.logger = NewLogger("connection/" + conn.id)
	conn.closed = false
	conn.closeMutex = &sync.Mutex{}

	conn.outgoing = make(chan OperationMessage)

	go conn.writeLoop()
	go conn.readLoop()

	conn.logger.Info("Created connection")

	return conn
}

func (conn *connection) ID() string {
	return conn.id
}

func (conn *connection) User() interface{} {
	return conn.user
}

func (conn *connection) SendData(opID string, data *DataMessagePayload) {
	msg := operationMessageForType(gqlData)
	msg.ID = opID
	msg.Payload = data
	conn.closeMutex.Lock()
	if !conn.closed {
		conn.outgoing <- msg
	}
	conn.closeMutex.Unlock()
}

func (conn *connection) SendError(err error) {
	msg := operationMessageForType(gqlError)
	msg.Payload = err.Error()
	conn.closeMutex.Lock()
	if !conn.closed {
		conn.outgoing <- msg
	}
	conn.closeMutex.Unlock()
}

func (conn *connection) sendOperationErrors(opID string, errs []error) {
	if conn.closed {
		return
	}
	msg := operationMessageForType(gqlError)
	msg.ID = opID
	msg.Payload = errs
	conn.closeMutex.Lock()
	if !conn.closed {
		conn.outgoing <- msg
	}
	conn.closeMutex.Unlock()
}

func (conn *connection) close() {
	// Close the write loop by closing the outgoing messages channels
	conn.closeMutex.Lock()
	conn.closed = true
	close(conn.outgoing)
	conn.closeMutex.Unlock()

	// Notify event handlers
	if conn.config.EventHandlers.Close != nil {
		conn.config.EventHandlers.Close(conn)
	}

	conn.logger.Info("Closed connection")
}

func (conn *connection) writeLoop() {
	// Close the WebSocket connection when leaving the write loop;
	// this ensures the read loop is also terminated and the connection
	// closed cleanly
	defer conn.ws.Close()

	for {
		select {
		// Take the next outgoing message from the channel
		case msg, ok := <-conn.outgoing:
			// Close the write loop when the outgoing messages channel is closed;
			// this will close the connection
			if !ok {
				return
			}

			conn.logger.WithFields(log.Fields{
				"msg": msg.String(),
			}).Debug("Send message")

			conn.ws.SetWriteDeadline(time.Now().Add(writeTimeout))

			// Send the message to the client; if this times out, the WebSocket
			// connection will be corrupt, hence we need to close the write loop
			// and the connection immediately
			if err := conn.ws.WriteJSON(msg); err != nil {
				conn.logger.WithFields(log.Fields{
					"err": err,
				}).Warn("Sending message failed")
				return
			}
		}
	}
}

func (conn *connection) readLoop() {
	// Close the WebSocket connection when leaving the read loop
	defer conn.ws.Close()

	conn.ws.SetReadLimit(readLimit)

	for {
		// Read the next message received from the client
		rawPayload := json.RawMessage{}
		msg := OperationMessage{
			Payload: &rawPayload,
		}
		err := conn.ws.ReadJSON(&msg)

		// If this causes an error, close the connection and read loop immediately;
		// see https://github.com/gorilla/websocket/blob/master/conn.go#L924 for
		// more information on why this is necessary
		if err != nil {
			conn.logger.WithFields(log.Fields{
				"reason": err,
			}).Warn("Closing connection")
			conn.close()
			return
		}

		conn.logger.WithFields(log.Fields{
			"id":   msg.ID,
			"type": msg.Type,
		}).Debug("Received message")

		switch msg.Type {

		// When the GraphQL WS connection is initiated, send an ACK back
		case gqlConnectionInit:
			data := InitMessagePayload{}
			if err := json.Unmarshal(rawPayload, &data); err != nil {
				conn.SendError(errors.New("Invalid GQL_CONNECTION_INIT payload"))
			} else {
				if conn.config.Authenticate != nil {
					user, err := conn.config.Authenticate(data.AuthToken)
					if err != nil {
						msg := operationMessageForType(gqlConnectionError)
						msg.Payload = fmt.Sprintf("Failed to authenticate user: %v", err)
						conn.outgoing <- msg
					} else {
						conn.user = user
						conn.outgoing <- operationMessageForType(gqlConnectionAck)
					}
				} else {
					conn.outgoing <- operationMessageForType(gqlConnectionAck)
				}
			}

		// Let event handlers deal with starting operations
		case gqlStart:
			if conn.config.EventHandlers.StartOperation != nil {
				data := StartMessagePayload{}
				if err := json.Unmarshal(rawPayload, &data); err != nil {
					conn.SendError(errors.New("Invalid GQL_START payload"))
				} else {
					errs := conn.config.EventHandlers.StartOperation(conn, msg.ID, &data)
					if errs != nil {
						conn.sendOperationErrors(msg.ID, errs)
					}
				}
			}

		// Let event handlers deal with stopping operations
		case gqlStop:
			if conn.config.EventHandlers.StopOperation != nil {
				conn.config.EventHandlers.StopOperation(conn, msg.ID)
			}

		// When the GraphQL WS connection is terminated by the client,
		// close the connection and close the read loop
		case gqlConnectionTerminate:
			conn.logger.Debug("Connection terminated by client")
			conn.close()
			return

		// GraphQL WS protocol messages that are not handled represent
		// a bug in our implementation; make this very obvious by logging
		// an error
		default:
			conn.logger.WithFields(log.Fields{
				"msg": msg.String(),
			}).Error("Unhandled message")
		}
	}
}
