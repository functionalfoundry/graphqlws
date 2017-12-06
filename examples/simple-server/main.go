package main

import (
	"net/http"

	"github.com/functionalfoundry/graphqlws"
	"github.com/graphql-go/graphql"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)
	log.Info("Starting example server on :8085")

	// GraphQL schema
	fields := graphql.Fields{
		"hello": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return "world", nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}
	schemaConfig := graphql.SchemaConfig{Query: graphql.NewObject(rootQuery)}
	schema, err := graphql.NewSchema(schemaConfig)

	if err != nil {
		log.WithField("err", err).Panic("GraphQL schema is invalid")
	}

	// Create subscription manager and GraphQL WS handler
	subscriptionManager := graphqlws.NewSubscriptionManager(&schema)
	websocketHandler := graphqlws.NewHandler(graphqlws.HandlerConfig{
		SubscriptionManager: subscriptionManager,
		Authenticate: func(token string) (interface{}, error) {
			return "Default user", nil
		},
	})

	// Serve the GraphQL WS endpoint
	http.Handle("/subscriptions", websocketHandler)
	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.WithField("err", err).Error("Failed to start server")
	}
}
