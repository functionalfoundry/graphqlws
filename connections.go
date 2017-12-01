package graphqlws

import (
	"encoding/json"
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

// OperationMessagePayload stores all parameters of an operation.
type OperationMessagePayload struct {
	Query         string                  `json:"query"`
	Variables     *map[string]interface{} `json:"variables"`
	OperationName *string                 `json:"operationName"`
}

// OperationMessage represents a standard GraphQL WebSocket message.
type OperationMessage struct {
	ID      *string                 `json:"id"`
	Type    *string                 `json:"type"`
	Payload OperationMessagePayload `json:"payload"`
}

func (msg *OperationMessage) String() string {
	s, _ := json.Marshal(msg)
	if s != nil {
		return string(s)
	}
	return "<invalid>"
}

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
	StartOperation func(Connection, *OperationMessage)

	// StopOperation is called whenever the client stops a previously
	// started GraphQL operation (typically a subscription). Event handlers
	// are expected to unregister the operation and stop sending result
	// data to the client.
	StopOperation func(Connection, *OperationMessage)
}

// Connection is an interface to represent GraphQL WebSocket connections.
// Each connection is associated with an ID that is unique to the server.
type Connection interface {
	// ID returns the unique ID of the connection.
	ID() string
}

// OperationDataPayload stores the data of a GraphQL execution result.
type OperationDataPayload struct {
	Data   map[string]interface{} `json:"data"`
	Errors []error                `json:"errors"`
}

// OperationData represents a GraphQL execution result for an operation.
type OperationData struct {
	ID      string               `json:"id"`
	Payload OperationDataPayload `json:"payload"`
}

func (data *OperationData) String() string {
	s, _ := json.Marshal(data)
	if s != nil {
		return string(s)
	}
	return "<invalid>"
}

/**
 * The default implementation of the Connection interface.
 */

type connection struct {
	id            string
	ws            *websocket.Conn
	eventHandlers *ConnectionEventHandlers
	logger        *log.Entry
	outgoing      chan *OperationMessage
}

func operationMessageForType(messageType string) *OperationMessage {
	return &OperationMessage{
		Type: &messageType,
	}
}

// NewConnection establishes a GraphQL WebSocket connection. It implements
// the GraphQL WebSocket protocol by managing its internal state and handling
// the client-server communication.
func NewConnection(ws *websocket.Conn, eventHandlers *ConnectionEventHandlers) Connection {
	conn := new(connection)
	conn.id = uuid.New().String()
	conn.ws = ws
	conn.eventHandlers = eventHandlers
	conn.logger = NewLogger("connection/" + conn.id)

	conn.outgoing = make(chan *OperationMessage)

	go conn.writeLoop()
	go conn.readLoop()

	conn.logger.Info("Created connection")

	return conn
}

func (conn *connection) ID() string {
	return conn.id
}

func (conn *connection) close() {
	// Close the write loop by closing the outgoing messages channel
	close(conn.outgoing)

	// Notify event handlers
	if conn.eventHandlers != nil {
		conn.eventHandlers.Close(conn)
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
		case message, ok := <-conn.outgoing:
			// Close the write loop when the outgoing messages channel is closed;
			// this will close the connection
			if !ok {
				return
			}

			conn.logger.WithFields(log.Fields{
				"message": message.String(),
			}).Debug("Send message")

			conn.ws.SetWriteDeadline(time.Now().Add(writeTimeout))

			// Send the message to the client; if this times out, the WebSocket
			// connection will be corrupt, hence we need to close the write loop
			// and the connection immediately
			if err := conn.ws.WriteJSON(message); err != nil {
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
		message := &OperationMessage{}
		err := conn.ws.ReadJSON(message)

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
			"message": message.String(),
		}).Debug("Received message")

		switch *message.Type {

		// When the GraphQL WS connection is initiated, send an ACK back
		case gqlConnectionInit:
			conn.outgoing <- operationMessageForType(gqlConnectionAck)

		// Let event handlers deal with starting operations
		case gqlStart:
			if conn.eventHandlers != nil {
				conn.eventHandlers.StartOperation(conn, message)
			}

		// Let event handlers deal with stopping operations
		case gqlStop:
			if conn.eventHandlers != nil {
				conn.eventHandlers.StopOperation(conn, message)
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
				"message": message.String(),
			}).Error("Unhandled message")
		}
	}
}
