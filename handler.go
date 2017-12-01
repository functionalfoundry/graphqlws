package graphqlws

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/gorilla/websocket"
)

// NewHandler creates a WebSocket handler for GraphQL WebSocket connections.
// This handler takes a SubscriptionManager and adds/removes subscriptions
// as they are started/stopped by the client.
func NewHandler(subscriptionManager SubscriptionManager) http.Handler {
	// Create a WebSocket upgrader that requires clients to implement
	// the "graphql-ws" protocol
	var upgrader = websocket.Upgrader{
		CheckOrigin:  func(r *http.Request) bool { return true },
		Subprotocols: []string{"graphql-ws"},
	}

	logger := NewLogger("handler")

	// Create a map (used like a set) to manage client connections
	var connections = make(map[Connection]bool)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Establish a WebSocket connection
			var ws, err = upgrader.Upgrade(w, r, nil)

			// Bail out if the WebSocket connection could not be established
			if err != nil {
				logger.Warn("Failed to establish WebSocket connection", err)
				return
			}

			// Close the connection early if it doesn't implement the graphql-ws protocol
			if ws.Subprotocol() != "graphql-ws" {
				logger.Warn("Connection does not implement the GraphQL WS protocol")
				ws.Close()
				return
			}

			// Establish a GraphQL WebSocket connection
			conn := NewConnection(ws, &ConnectionEventHandlers{
				Close: func (conn Connection) {
					logger.WithFields(log.Fields{
						"conn": conn.ID(),
					}).Debug("Closing connection")

					subscriptionManager.RemoveSubscriptions(conn)

					delete(connections, conn)
				},
				StartOperation: func (conn Connection, msg *OperationMessage) {
					logger.WithFields(log.Fields{
						"conn": conn.ID(),
						"op": *msg.ID,
					}).Debug("Start operation")

					subscriptionManager.AddSubscription(conn, &Subscription{
						ID: *msg.ID,
						Query: msg.Payload.Query,
						Variables: msg.Payload.Variables,
						OperationName: msg.Payload.OperationName,
						SendData: func(subscription *Subscription, data *OperationData) {
							logger.WithFields(log.Fields{
								"conn": conn.ID(),
								"data": data.String(),
							}).Debug("Send subscription update to client")
						},
					})
				},
				StopOperation: func (conn Connection, msg *OperationMessage) {
					subscriptionManager.RemoveSubscription(conn, &Subscription{
						ID: *msg.ID,
					})
				},
			})
			connections[conn] = true
		},
	)
}
