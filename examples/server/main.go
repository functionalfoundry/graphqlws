package main

import (
	"net/http"

	"github.com/functionalfoundry/graphqlws"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"

	log "github.com/sirupsen/logrus"
)

type document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

var documents = []document{
	{Title: "My diary", Content: "Today I had fun with graphqlws"},
	{Title: "Todo", Content: "Add a complete example"},
}

var schema graphql.Schema
var subscriptionManager graphqlws.SubscriptionManager

func main() {
	log.SetLevel(log.InfoLevel)
	log.Info("Starting example server on :8085")

	var documentType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Document",
			Fields: graphql.Fields{
				"title": &graphql.Field{
					Type: graphql.String,
				},
				"content": &graphql.Field{
					Type: graphql.String,
				},
			},
		},
	)

	var queryType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"document": &graphql.Field{
					Type: documentType,
					Args: graphql.FieldConfigArgument{
						"id": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id := p.Args["id"].(int)
						return documents[id], nil
					},
				},
			},
		},
	)

	var mutationType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Mutation",
			Fields: graphql.Fields{
				"updateDocument": &graphql.Field{
					Type: documentType,
					Args: graphql.FieldConfigArgument{
						"id": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.Int),
						},
						"title": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.String),
						},
						"content": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.String),
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id := p.Args["id"].(int)
						documents[id].Title = p.Args["title"].(string)
						documents[id].Content = p.Args["content"].(string)

						for _, subs := range subscriptionManager.Subscriptions() {
							for _, sub := range subs {
								// JSON interface is float64
								var subID int = int(sub.Variables["id"].(float64))

								if id == subID {
									params := graphql.Params{
										Schema:         schema,
										RequestString:  sub.Query,
										VariableValues: sub.Variables,
										OperationName:  sub.OperationName,
									}
									result := graphql.Do(params)

									data := graphqlws.DataMessagePayload{
										Data: result.Data,
										Errors: graphqlws.ErrorsFromGraphQLErrors(
											result.Errors,
										),
									}

									sub.SendData(&data)
								}
							}
						}

						return documents[id], nil
					},
				},
			},
		},
	)

	var subscriptionType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"documentUpdates": &graphql.Field{
					Type: documentType,
					Args: graphql.FieldConfigArgument{
						"id": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id := p.Args["id"].(int)
						return documents[id], nil
					},
				},
			},
		},
	)

	schemaConfig := graphql.SchemaConfig{
		Query:        queryType,
		Mutation:     mutationType,
		Subscription: subscriptionType,
	}

	var err error
	schema, err = graphql.NewSchema(schemaConfig)
	if err != nil {
		log.WithField("err", err).Panic("GraphQL schema is invalid")
	}

	subscriptionManager = graphqlws.NewSubscriptionManager(&schema)
	websocketHandler := graphqlws.NewHandler(graphqlws.HandlerConfig{
		SubscriptionManager: subscriptionManager,
		Authenticate: func(token string) (interface{}, error) {
			return "Default user", nil
		},
	})

	graphqlHandler := handler.New(&handler.Config{
		Schema:                &schema,
		Pretty:                true,
		GraphiQL:              true,
		Endpoint:              "http://localhost:8085",
		SubscriptionsEndpoint: "ws://localhost:8085/subscriptions",
	})

	http.Handle("/", graphqlHandler)
	http.Handle("/subscriptions", websocketHandler)
	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.WithField("err", err).Error("Failed to start server")
	}
}
