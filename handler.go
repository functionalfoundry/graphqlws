package graphqlws

import (
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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
				Close: func(conn Connection) {
					logger.WithFields(log.Fields{
						"conn": conn.ID(),
					}).Debug("Closing connection")

					subscriptionManager.RemoveSubscriptions(conn)

					delete(connections, conn)
				},
				StartOperation: func(
					conn Connection,
					opID string,
					data *StartMessagePayload,
				) []error {
					logger.WithFields(log.Fields{
						"conn": conn.ID(),
						"op":   opID,
					}).Debug("Start operation")

					return subscriptionManager.AddSubscription(conn, &Subscription{
						ID:            opID,
						Query:         data.Query,
						Variables:     data.Variables,
						OperationName: data.OperationName,
						SendData: func(subscription *Subscription, data *DataMessagePayload) {
							conn.SendData(opID, data)
						},
					})
				},
				StopOperation: func(conn Connection, opID string) {
					subscriptionManager.RemoveSubscription(conn, &Subscription{
						ID: opID,
					})
				},
			})
			connections[conn] = true
		},
	)
}
