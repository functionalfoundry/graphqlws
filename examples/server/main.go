package main

import (
	"net/http"

	"github.com/functionalfoundry/graphqlws"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"

	log "github.com/sirupsen/logrus"
)

type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

var documents = []Document{
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
						"docId": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						docID := p.Args["docId"].(int)
						return documents[docID], nil
					},
				},
			},
		},
	)

	var mutationType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "SomeMutation",
			Fields: graphql.Fields{
				"updateDocument": &graphql.Field{
					Type: documentType,
					Args: graphql.FieldConfigArgument{
						"docId": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
						"title": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
						"content": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {

						docID := p.Args["docId"].(int)
						documents[docID].Title = p.Args["title"].(string)
						documents[docID].Title = p.Args["title"].(string)

						for _, subscriptions := range subscriptionManager.Subscriptions() {
							for _, subscription := range subscriptions {

								// JSON interface is float64
								var subdocID int = int(subscription.Variables["docId"].(float64))

								if docID == subdocID {

									params := graphql.Params{
										Schema:         schema,
										RequestString:  subscription.Query,
										VariableValues: subscription.Variables,
										OperationName:  subscription.OperationName,
									}
									result := graphql.Do(params)

									data := graphqlws.DataMessagePayload{
										Data:   result.Data,
										Errors: graphqlws.ErrorsFromGraphQLErrors(result.Errors),
									}

									subscription.SendData(subscription, &data)
								}
							}
						}

						return documents[docID], nil
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
						"docId": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						docID := p.Args["docId"].(int)
						return documents[docID], nil
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
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})

	http.Handle("/", graphqlHandler)
	http.Handle("/subscriptions", websocketHandler)
	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.WithField("err", err).Error("Failed to start server")
	}
}
