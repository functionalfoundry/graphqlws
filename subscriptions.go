package graphqlws

import (
	log "github.com/sirupsen/logrus"
)

// SubscriptionSendDataFunc is a function that sends updated data
// for a specific subscription to the corresponding subscriber.
type SubscriptionSendDataFunc func(*Subscription, *OperationData)

// Subscription holds all information about a GraphQL subscription
// made by a client, including a function to send data back to the
// client when there are updates to the subscription query result.
type Subscription struct {
	ID            string
	Query         string
	Variables     *map[string]interface{}
	OperationName *string
	SendData      SubscriptionSendDataFunc
}

// SubscriptionManager provides a high-level interface to managing
// and accessing the subscriptions made by GraphQL WS clients.
type SubscriptionManager interface {
	// AddSubscription adds a new subscription to the manager.
	AddSubscription(Connection, *Subscription)

	// RemoveSubscription removes a subscription from the manager.
	RemoveSubscription(Connection, *Subscription)

	// RemoveSubscriptions removes all subscriptions of a client connection.
	RemoveSubscriptions(Connection)
}

/**
 * The default implementation of the SubscriptionManager interface.
 */

type subscriptionManager struct {
	subscriptions map[Connection]map[string]*Subscription
	logger        *log.Entry
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() SubscriptionManager {
	manager := new(subscriptionManager)
	manager.subscriptions = make(map[Connection]map[string]*Subscription)
	manager.logger = NewLogger("subscriptions")
	return manager
}

func (m *subscriptionManager) AddSubscription(
	conn Connection,
	subscription *Subscription,
) {
	m.logger.WithFields(log.Fields{
		"conn":         conn.ID(),
		"subscription": subscription.ID,
	}).Info("Add subscription")

	// Allocate the connection's map of subscription IDs to
	// subscriptions on demand
	if m.subscriptions[conn] == nil {
		m.subscriptions[conn] = make(map[string]*Subscription)
	}

	// Add the subscription if it hasn't already been added
	if m.subscriptions[conn][subscription.ID] != nil {
		m.logger.WithFields(log.Fields{
			"conn":         conn.ID(),
			"subscription": subscription.ID,
		}).Warn("Cannot register subscription twice")
	} else {
		m.subscriptions[conn][subscription.ID] = subscription
	}
}

func (m *subscriptionManager) RemoveSubscription(
	conn Connection,
	subscription *Subscription,
) {
	m.logger.WithFields(log.Fields{
		"conn":         conn.ID(),
		"subscription": subscription.ID,
	}).Info("Remove subscription")

	// Remove the subscription from its connections' subscriton map
	delete(m.subscriptions[conn], subscription.ID)
}

func (m *subscriptionManager) RemoveSubscriptions(conn Connection) {
	m.logger.WithFields(log.Fields{
		"conn": conn.ID(),
	}).Info("Remove subscriptions")

	// Only remove subscriptions if we know the connection
	if m.subscriptions[conn] != nil {
		// Remove subscriptions one by one
		for opID := range m.subscriptions[conn] {
			m.RemoveSubscription(conn, m.subscriptions[conn][opID])
		}

		// Remove the connection's subscription map altogether
		delete(m.subscriptions, conn)
	}
}
