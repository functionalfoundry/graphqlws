package graphqlws

import (
	"errors"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	log "github.com/sirupsen/logrus"
)

// ErrorsFromGraphQLErrors convert from GraphQL errors to regular errors.
func ErrorsFromGraphQLErrors(errors []gqlerrors.FormattedError) []error {
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
	Document      *ast.Document
	Fields        []string
	Connection    Connection
	SendData      SubscriptionSendDataFunc
}

// MatchesField returns true if the subscription is for data that
// belongs to the given field.
func (s *Subscription) MatchesField(field string) bool {
	if s.Document == nil || len(s.Fields) == 0 {
		return false
	}

	// The subscription matches the field if any of the queries have
	// the same name as the field
	for _, name := range s.Fields {
		if name == field {
			return true
		}
	}
	return false
}

// ConnectionSubscriptions defines a map of all subscriptions of
// a connection by their IDs.
type ConnectionSubscriptions map[string]*Subscription

// Subscriptions defines a map of connections to a map of
// subscription IDs to subscriptions.
type Subscriptions map[Connection]ConnectionSubscriptions

// SubscriptionListener defines an interface for components that
// are to be notified of added/removed subscriptions.
type SubscriptionListener interface {
	// SubscriptionAdded is called whenever a new subscription is added.
	SubscriptionAdded(Connection, *Subscription)

	// SubscriptionRemoved is called whenever a subscription is removed.
	SubscriptionRemoved(Connection, *Subscription)
}

// SubscriptionManager provides a high-level interface to managing
// and accessing the subscriptions made by GraphQL WS clients.
type SubscriptionManager interface {
	// Subscriptions returns all registered subscriptions, grouped
	// by connection.
	Subscriptions() Subscriptions

	// AddSubscription adds a new subscription to the manager.
	AddSubscription(Connection, *Subscription) []error

	// RemoveSubscription removes a subscription from the manager.
	RemoveSubscription(Connection, *Subscription)

	// RemoveSubscriptions removes all subscriptions of a client connection.
	RemoveSubscriptions(Connection)

	// AddSubscriptionListener adds a listener to be notified when subscriptions
	// are added or removed.
	AddSubscriptionListener(SubscriptionListener)

	// RemoveSubscriptionListener removes a previously added listener.
	RemoveSubscriptionListener(SubscriptionListener)
}

/**
 * The default implementation of the SubscriptionManager interface.
 */

type subscriptionManager struct {
	subscriptions Subscriptions
	schema        *graphql.Schema
	logger        *log.Entry
	listeners     map[SubscriptionListener]bool
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager(schema *graphql.Schema) SubscriptionManager {
	return &subscriptionManager{
		subscriptions: make(Subscriptions),
		logger:        NewLogger("subscriptions"),
		schema:        schema,
		listeners:     map[SubscriptionListener]bool{},
	}
}

func (m *subscriptionManager) Subscriptions() Subscriptions {
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

	if errors := validateSubscription(subscription); len(errors) > 0 {
		m.logger.WithField("errors", errors).Warn("Failed to add invalid subscription")
		return errors
	}

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
		return ErrorsFromGraphQLErrors(validation.Errors)
	}

	// Remember the query document for later
	subscription.Document = document

	// Extract query names from the document (typically, there should only be one)
	subscription.Fields = subscriptionFieldNamesFromDocument(document)

	// Allocate the connection's map of subscription IDs to
	// subscriptions on demand
	if m.subscriptions[conn] == nil {
		m.subscriptions[conn] = make(ConnectionSubscriptions)
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

	// Notify listeners
	m.notifySubscriptionAdded(conn, subscription)

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

	// Notify listeners
	m.notifySubscriptionRemoved(conn, subscription)

	// Remove the subscription from its connections' subscription map
	delete(m.subscriptions[conn], subscription.ID)

	// Remove the connection as well if there are no subscriptions left
	if len(m.subscriptions[conn]) == 0 {
		delete(m.subscriptions, conn)
	}
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

func (m *subscriptionManager) AddSubscriptionListener(listener SubscriptionListener) {
	m.listeners[listener] = true
}

func (m *subscriptionManager) RemoveSubscriptionListener(listener SubscriptionListener) {
	delete(m.listeners, listener)
}

func (m *subscriptionManager) notifySubscriptionAdded(
	conn Connection,
	subscription *Subscription,
) {
	for listener := range m.listeners {
		listener.SubscriptionAdded(conn, subscription)
	}
}

func (m *subscriptionManager) notifySubscriptionRemoved(
	conn Connection,
	subscription *Subscription,
) {
	for listener := range m.listeners {
		listener.SubscriptionRemoved(conn, subscription)
	}
}

func validateSubscription(s *Subscription) []error {
	errs := []error{}

	if s.ID == "" {
		errs = append(errs, errors.New("Subscription ID is empty"))
	}

	if s.Connection == nil {
		errs = append(errs, errors.New("Subscription is not associated with a connection"))
	}

	if s.Query == "" {
		errs = append(errs, errors.New("Subscription query is empty"))
	}

	if s.SendData == nil {
		errs = append(errs, errors.New("Subscription has no SendData function set"))
	}

	return errs
}
