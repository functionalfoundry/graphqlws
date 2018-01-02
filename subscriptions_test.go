package graphqlws_test

import (
	"testing"

	"github.com/functionalfoundry/graphqlws"
	"github.com/graphql-go/graphql"
	log "github.com/sirupsen/logrus"
)

// Mock connection

type mockWebSocketConnection struct {
	user string
	id   string
}

func (c *mockWebSocketConnection) ID() string {
	return c.id
}

func (c *mockWebSocketConnection) User() interface{} {
	return c.user
}

func (c *mockWebSocketConnection) SendData(
	opID string,
	data *graphqlws.DataMessagePayload,
) {
	// Do nothing
}

func (c *mockWebSocketConnection) SendError(err error) {
	// Do nothing
}

// Tests

func TestMain(m *testing.M) {
	log.SetLevel(log.ErrorLevel)
}

func TestSubscriptions_NewSubscriptionManagerCreatesInstance(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{})

	sm := graphqlws.NewSubscriptionManager(&schema)
	if sm == nil {
		t.Fatal("NewSubscriptionManager fails in creating a new instance")
	}
}

func TestSubscriptions_SubscriptionsAreEmptyInitially(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{})
	sm := graphqlws.NewSubscriptionManager(&schema)

	if len(sm.Subscriptions()) > 0 {
		t.Fatal("The subscriptions of SubscriptionManager are not empty initially")
	}
}

func TestSubscriptions_AddingInvalidSubscriptionsFails(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{})
	sm := graphqlws.NewSubscriptionManager(&schema)

	conn := mockWebSocketConnection{
		id: "1",
	}

	// Try adding a subscription with nothing set
	errors := sm.AddSubscription(&conn, &graphqlws.Subscription{})

	if len(errors) == 0 {
		t.Error("AddSubscription does not fail when adding an empty subscription")
	}

	if len(sm.Subscriptions()) > 0 {
		t.Fatal("AddSubscription unexpectedly adds empty subscriptions")
	}

	// Try adding a subscription with an invalid query
	errors = sm.AddSubscription(&conn, &graphqlws.Subscription{
		Query: "<<<Fooo>>>",
	})

	if len(errors) == 0 {
		t.Error("AddSubscription does not fail when adding an invalid subscription")
	}

	if len(sm.Subscriptions()) > 0 {
		t.Fatal("AddSubscription unexpectedly adds invalid subscriptions")
	}

	// Try adding a subscription with a query that doesn't match the schema
	errors = sm.AddSubscription(&conn, &graphqlws.Subscription{
		Query: "subscription { foo }",
	})

	if len(errors) == 0 {
		t.Error("AddSubscription doesn't fail if the query doesn't match the schema")
	}

	if len(sm.Subscriptions()) > 0 {
		t.Fatal("AddSubscription unexpectedly adds invalid subscriptions")
	}
}

func TestSubscriptions_AddingValidSubscriptionsWorks(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"users": &graphql.Field{
					Type: graphql.NewList(graphql.String),
				},
			},
		})})
	sm := graphqlws.NewSubscriptionManager(&schema)

	conn := mockWebSocketConnection{
		id: "1",
	}

	// Add a valid subscription
	sub1 := graphqlws.Subscription{
		ID:         "1",
		Connection: &conn,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	errors := sm.AddSubscription(&conn, &sub1)

	if len(errors) > 0 {
		t.Error(
			"AddSubscription fails adding valid subscriptions. Unexpected errors:",
			errors,
		)
	}

	if len(sm.Subscriptions()) != 1 ||
		len(sm.Subscriptions()[&conn]) != 1 ||
		sm.Subscriptions()[&conn]["1"] != &sub1 {
		t.Fatal("AddSubscription doesn't add valid subscriptions properly")
	}

	// Add another valid subscription
	sub2 := graphqlws.Subscription{
		ID:         "2",
		Connection: &conn,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	errors = sm.AddSubscription(&conn, &sub2)
	if len(errors) > 0 {
		t.Error(
			"AddSubscription fails adding valid subscriptions.",
			"Unexpected errors:", errors,
		)
	}
	if len(sm.Subscriptions()) != 1 ||
		len(sm.Subscriptions()[&conn]) != 2 ||
		sm.Subscriptions()[&conn]["1"] != &sub1 ||
		sm.Subscriptions()[&conn]["2"] != &sub2 {
		t.Fatal("AddSubscription doesn't add valid subscriptions properly")
	}
}

func TestSubscriptions_AddingSubscriptionsTwiceFails(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"users": &graphql.Field{
					Type: graphql.NewList(graphql.String),
				},
			},
		})})
	sm := graphqlws.NewSubscriptionManager(&schema)

	conn := mockWebSocketConnection{
		id: "1",
	}

	// Add a valid subscription
	sub := graphqlws.Subscription{
		ID:         "1",
		Connection: &conn,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn, &sub)

	// Try adding the subscription for a second time
	errors := sm.AddSubscription(&conn, &sub)

	if len(errors) == 0 {
		t.Error(
			"AddSubscription doesn't fail when adding subscriptions a second time.",
			"Unexpected errors:", errors,
		)
	}

	if len(sm.Subscriptions()) != 1 ||
		len(sm.Subscriptions()[&conn]) != 1 ||
		sm.Subscriptions()[&conn]["1"] != &sub {
		t.Fatal("AddSubscription unexpectedly adds subscriptions twice")
	}
}

func TestSubscriptions_RemovingSubscriptionsWorks(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"users": &graphql.Field{
					Type: graphql.NewList(graphql.String),
				},
			},
		})})
	sm := graphqlws.NewSubscriptionManager(&schema)

	conn := mockWebSocketConnection{
		id: "1",
	}

	// Add two valid subscriptions
	sub1 := graphqlws.Subscription{
		ID:         "1",
		Connection: &conn,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn, &sub1)
	sub2 := graphqlws.Subscription{
		ID:         "2",
		Connection: &conn,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn, &sub2)

	// Remove the first subscription
	sm.RemoveSubscription(&conn, &sub1)

	// Verify that only one subscription is left
	if len(sm.Subscriptions()) != 1 || len(sm.Subscriptions()[&conn]) != 1 {
		t.Error("RemoveSubscription does not remove subscriptions")
	}

	// Remove the second subscription
	sm.RemoveSubscription(&conn, &sub2)

	// Verify that there are no subscriptions left
	if len(sm.Subscriptions()) != 0 {
		t.Error("RemoveSubscription does not remove subscriptions")
	}
}

func TestSubscriptions_RemovingSubscriptionsOfAConnectionWorks(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"users": &graphql.Field{
					Type: graphql.NewList(graphql.String),
				},
			},
		})})
	sm := graphqlws.NewSubscriptionManager(&schema)

	conn1 := mockWebSocketConnection{id: "1"}
	conn2 := mockWebSocketConnection{id: "2"}

	// Add four valid subscriptions
	sub1 := graphqlws.Subscription{
		ID:         "1",
		Connection: &conn1,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn1, &sub1)
	sub2 := graphqlws.Subscription{
		ID:         "2",
		Connection: &conn1,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn1, &sub2)
	sub3 := graphqlws.Subscription{
		ID:         "1",
		Connection: &conn2,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn2, &sub3)
	sub4 := graphqlws.Subscription{
		ID:         "2",
		Connection: &conn2,
		Query:      "subscription { users }",
		SendData: func(msg *graphqlws.DataMessagePayload) {
			// Do nothing
		},
	}
	sm.AddSubscription(&conn2, &sub4)

	// Remove subscriptions of the first connection
	sm.RemoveSubscriptions(&conn1)

	// Verify that only the subscriptions of the second connection remain
	if len(sm.Subscriptions()) != 1 ||
		len(sm.Subscriptions()[&conn2]) != 2 ||
		sm.Subscriptions()[&conn1] != nil {
		t.Error("RemoveSubscriptions doesn't remove subscriptions of connections")
	}

	// Remove subscriptions of the second connection
	sm.RemoveSubscriptions(&conn2)

	// Verify that there are no subscriptions left
	if len(sm.Subscriptions()) != 0 {
		t.Error("RemoveSubscriptions doesn't remove subscriptions of connections")
	}
}
