**Note: We're looking for a new maintainer for `graphqlws`. Please reach out via jannis@thegraph.com if you're interested.**

---

# graphqlws

Implementation of the [GraphQL over WebSocket protocol] in Go.
Brought to you by [Functional Foundry](https://functionalfoundry.com).

[API Documentation](https://godoc.org/github.com/functionalfoundry/graphqlws)

[![Build Status](https://travis-ci.org/functionalfoundry/graphqlws.svg?branch=master)](https://travis-ci.org/functionalfoundry/graphqlws)
[![Go Report](https://goreportcard.com/badge/github.com/functionalfoundry/graphqlws)](https://goreportcard.com/report/github.com/functionalfoundry/graphqlws)

## Getting started

1. Install dependencies:
   ```sh
   go get github.com/sirupsen/logrus
   go get github.com/x-cray/logrus-prefixed-formatter
   go get github.com/google/uuid
   go get github.com/gorilla/websocket
   go get github.com/graphql-go/graphql
   ```
2. Clone the repository:
   ```sh
   mkdir -p "$GOPATH/src/github.com/functionalfoundry"
   cd "$GOPATH/src/github.com/functionalfoundry"
   git clone https://github.com/functionalfoundry/graphqlws
   ```
4. Run the tests:
   ```sh
   cd graphqlws
   go test
   ```
3. Run the example server:
   ```sh
   go run graphqlws/examples/server
   ```

## Usage

### Setup

```go
package main

import (
	"net/http"

	"github.com/functionalfoundry/graphqlws"
	"github.com/graphql-go/graphql"
)

func main() {
	// Create a GraphQL schema
	schema, err := graphql.NewSchema(...)

	// Create a subscription manager
	subscriptionManager := graphqlws.NewSubscriptionManager(&schema)

	// Create a WebSocket/HTTP handler
	graphqlwsHandler := graphqlws.NewHandler(graphqlws.HandlerConfig{
		// Wire up the GraphqL WebSocket handler with the subscription manager
		SubscriptionManager: subscriptionManager,

		// Optional: Add a hook to resolve auth tokens into users that are
		// then stored on the GraphQL WS connections
		Authenticate: func(authToken string) (interface{}, error) {
			// This is just a dumb example
			return "Joe", nil
		},
	})

	// The handler integrates seamlessly with existing HTTP servers
	http.Handle("/subscriptions", graphqlwsHandler)
	http.ListenAndServe(":8080", nil)
}
```

### Working with subscriptions

```go
// This assumes you have access to the above subscription manager
subscriptions := subscriptionManager.Subscriptions()

for conn, _ := range subscriptions {
	// Things you have access to here:
	conn.ID()   // The connection ID
	conn.User() // The user returned from the Authenticate function

	for _, subscription := range subscriptions[conn] {
		// Things you have access to here:
		subscription.ID            // The subscription ID (unique per conn)
		subscription.OperationName // The name of the operation
		subscription.Query         // The subscription query/queries string
		subscription.Variables     // The subscription variables
		subscription.Document      // The GraphQL AST for the subscription
		subscription.Fields        // The names of top-level queries
		subscription.Connection    // The GraphQL WS connection

		// Prepare an execution context for running the query
		ctx := context.Context()

		// Re-execute the subscription query
		params := graphql.Params{
			Schema:         schema, // The GraphQL schema
			RequestString:  subscription.Query,
			VariableValues: subscription.Variables,
			OperationName:  subscription.OperationName,
			Context:        ctx,
		}
		result := graphql.Do(params)

		// Send query results back to the subscriber at any point
		data := graphqlws.DataMessagePayload{
			// Data can be anything (interface{})
			Data:   result.Data,
			// Errors is optional ([]error)
			Errors: graphqlws.ErrorsFromGraphQLErrors(result.Errors),
		}
		subscription.SendData(&data)
	}
}
```

### Logging

`graphqlws` uses [logrus](https://github.com/sirupsen/logrus) for logging.
In the future we might remove those logs entirely to leave logging entirely to developers
using `graphqlws`. Given the current solution, you can control the logging level of
`graphqlws` by setting it through `logrus`:

```go
import (
  log "github.com/sirupsen/logrus"
)

...

log.SetLevel(log.WarnLevel)
```

## License

Copyright © 2017-2019 Functional Foundry, LLC.

Licensed under the [MIT License](LICENSE.md).

[graphql over websocket protocol]: https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md
