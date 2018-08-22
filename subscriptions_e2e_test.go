package graphqlws_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/functionalfoundry/graphqlws"
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	log "github.com/sirupsen/logrus"
)

var wsMountPath = "/subscriptions"
var subscriptionName = "StaticString"

func TestSubscriptions(t *testing.T) {

	testedValue := "1231331313"
	numberOfMessages := 3

	log.Infof("Building schema")
	schema, err := buildSchema()
	if err != nil {
		t.Errorf("could not build graphql schema: " + err.Error())
		t.FailNow()
	}

	subscriptionManager := graphqlws.NewSubscriptionManager(schema)

	srv := startServer(subscriptionManager)
	defer srv.Close()

	port := ":" + strings.Split(srv.URL,":")[2]
	log.Infof("Starting server on port: %s", port)


	graphqlWsHeader := http.Header{}
	graphqlWsHeader["Sec-WebSocket-Protocol"] = []string{"graphql-ws"}

	webSocketConnectUrl := "ws://localhost" + port + wsMountPath
	log.Infof("Connecting WebSocket client to %s", webSocketConnectUrl)
	webSocketClient, resp, err := websocket.DefaultDialer.Dial(webSocketConnectUrl, graphqlWsHeader)
	if err != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		responseBody := buf.String()
		t.Errorf("couldn't connect to websocket resource: '%s', response: '%s'", err.Error(), responseBody)
		t.FailNow()
	}
	defer webSocketClient.Close()

	queryMessage := fmt.Sprintf(`{
	  "id": "1",
	  "type": "start",
	  "payload": {
		"variables": {},
		"extensions": {},
		"operationName": null,
		"query": "subscription { %s { payload } }"
	  }
	}`, subscriptionName)


	log.Infof("Subscribing for %s events", subscriptionName)
	err = webSocketClient.WriteMessage(websocket.TextMessage, []byte(queryMessage))
	if err != nil {
		t.Errorf("could not subscribe to events: %s", err.Error())
		t.FailNow()
	}

	messageChannel := make(chan string, numberOfMessages)

	log.Infof("Listening for websocket events")
	go listenForMessages(webSocketClient, messageChannel, numberOfMessages)
	time.Sleep(500 * time.Millisecond)

	payload := map[string]interface{} {
		"payload" : testedValue,
	}

	for i:= 0; i<numberOfMessages; i++ {
		triggerSubscription(payload, schema, subscriptionManager)
	}

	expectedMessage := fmt.Sprintf("{\"id\":\"1\",\"type\":\"data\",\"payload\":{\"data\":{\"%s\":{\"payload\":\"%s\"}},\"errors\":null}}", subscriptionName, testedValue)

	for i:=0; i<numberOfMessages; i++ {
		receivedMessage := <- messageChannel
		receivedMessage = strings.Replace(receivedMessage, "\n", "", -1)
		if receivedMessage != expectedMessage {
			t.Errorf("unexpected value received: '%s', expected: '%s'", receivedMessage, expectedMessage)
		}
	}

}

func listenForMessages(webSocketClient *websocket.Conn, messageChannel chan string, maxNumberOfMessages int) {
	messagesProcessed := 0
	for {
		_, message, err := webSocketClient.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		messagesProcessed++
		messageChannel <- string(message)
		if messagesProcessed == maxNumberOfMessages {
			return
		}
	}
}

func triggerSubscription(payload map[string]interface{}, schema *graphql.Schema, manager graphqlws.SubscriptionManager) error {
	allSubscriptions := manager.Subscriptions()

	sameQuerySubscriptionsMap := make(map[string][]*graphqlws.Subscription)
	for connection := range allSubscriptions {
		for _, subscription := range allSubscriptions[connection] {
			for _, field := range subscription.Fields {
				if field == subscriptionName {
					sameQuerySubscriptionsMap[subscription.Query] = append(sameQuerySubscriptionsMap[subscription.Query], subscription)
				}
			}
		}
	}

	for _, sameQuerySubscriptions := range sameQuerySubscriptionsMap {

		var subscriptionData *graphqlws.DataMessagePayload
		for _, subscription := range sameQuerySubscriptions {
			if subscriptionData == nil {
				ctx := context.Background()

				params := graphql.Params{
					Schema:         *schema,
					RequestString:  subscription.Query,
					VariableValues: subscription.Variables,
					OperationName:  subscription.OperationName,
					Context:        ctx,
					RootObject:     payload,
				}

				result := graphql.Do(params)

				subscriptionData = &graphqlws.DataMessagePayload{
					Data:   result.Data,
					Errors: graphqlws.ErrorsFromGraphQLErrors(result.Errors),
				}
			}
			subscription.SendData(subscriptionData)
		}
	}
	return nil
}

func startServer(subscriptionManager graphqlws.SubscriptionManager) *httptest.Server {

	websocketHandler := graphqlws.NewHandler(graphqlws.HandlerConfig{
		SubscriptionManager: subscriptionManager,
	})

	srv := httptest.NewServer(websocketHandler)
	time.Sleep(500 * time.Millisecond)
	return srv
}

func defineSubscriptionSchema() graphql.Fields {
	subscriptions := make(graphql.Fields)

	randomStringSubscription := &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "StaticStringSubscriptionPayload",
			Fields: graphql.Fields{
				"payload": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
		}),
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return p.Source, nil
		},
	}

	subscriptions[subscriptionName] = randomStringSubscription

	return subscriptions
}

func buildSchema() (*graphql.Schema, error) {
	fields := graphql.Fields{
		"hello": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return "world", nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}
	rootSubscription := graphql.ObjectConfig{Name: "RootSubscription", Fields: defineSubscriptionSchema()}
	schemaConfig := graphql.SchemaConfig{
		Query:        graphql.NewObject(rootQuery),
		Subscription: graphql.NewObject(rootSubscription),
	}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		return nil, err
	}
	return &schema, nil
}
