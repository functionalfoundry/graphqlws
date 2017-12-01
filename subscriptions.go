package graphqlws

import (
	"errors"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/parser"
	log "github.com/sirupsen/logrus"
)

// Convert from validation errors to regular errors
func translateValidationErrors(errors []gqlerrors.FormattedError) []error {
	out := make([]error, len(errors))
	for i := range errors {
		out[i] = errors[i]
	}
	return out
}

// SubscriptionSendDataFunc is a function that sends updated data
// for a specific subscription to the corresponding subscriber.
type SubscriptionSendDataFunc func(*Subscription, *DataMessagePayload)

// Subscription holds all information about a GraphQL subscription
// made by a client, including a function to send data back to the
// client when there are updates to the subscription query result.
type Subscription struct {
	ID            string
	Query         string
	Variables     map[string]interface{}
	OperationName string
	SendData      SubscriptionSendDataFunc
}

// SubscriptionManager provides a high-level interface to managing
// and accessing the subscriptions made by GraphQL WS clients.
type SubscriptionManager interface {
	// Subscriptions returns all registered subscriptions, grouped
	// by connection.
	Subscriptions() map[Connection]map[string]*Subscription

	// AddSubscription adds a new subscription to the manager.
	AddSubscription(Connection, *Subscription) []error

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
	schema        *graphql.Schema
	logger        *log.Entry
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager(schema *graphql.Schema) SubscriptionManager {
	manager := new(subscriptionManager)
	manager.subscriptions = make(map[Connection]map[string]*Subscription)
	manager.logger = NewLogger("subscriptions")
	manager.schema = schema
	return manager
}

func (m *subscriptionManager) Subscriptions() map[Connection]map[string]*Subscription {
	return m.subscriptions
}

func (m *subscriptionManager) AddSubscription(
	conn Connection,
	subscription *Subscription,
) []error {
	m.logger.WithFields(log.Fields{
		"conn":         conn.ID(),
		"subscription": subscription.ID,
	}).Info("Add subscription")

	// Parse the subscription query
	document, err := parser.Parse(parser.ParseParams{
		Source: subscription.Query,
	})
	if err != nil {
		m.logger.WithField("err", err).Warn("Failed to parse subscription query")
		return []error{err}
	}

	// Validate the query document
	validation := graphql.ValidateDocument(m.schema, document, nil)
	if !validation.IsValid {
		m.logger.WithFields(log.Fields{
			"errors": validation.Errors,
		}).Warn("Failed to validate subscription query")
		return translateValidationErrors(validation.Errors)
	}

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
		return []error{errors.New("Cannot register subscription twice")}
	}

	m.subscriptions[conn][subscription.ID] = subscription

	return nil
}

func (m *subscriptionManager) RemoveSubscription(
	conn Connection,
	subscription *Subscription,
) {
	m.logger.WithFields(log.Fields{
		"conn":         conn.ID(),
		"subscription": subscription.ID,
	}).Info("Remove subscription")

	// Remove the subscription from its connections' subscription map
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
