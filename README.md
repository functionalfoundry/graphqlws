# graphqlws

Implementation of the [GraphQL over WebSocket protocol] in Go.

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
   mkdir -p "$GOPATH/github.com/functionalfoundry"
   cd "$GOPATH/github.com/functionalfoundry"
   git clone https://github.com/functionalfoundry/graphqlws
   ```
3. Run the example server:
   ```sh
   cd graphqlws
   go run graphqlws/examples/server
   ```

## Usage

### Setup

```go
package main

import (
  "net/http"

  "github.com/functionalfoundry/graphqlws"
)

func main() {
  // Create a subscription manager
  subscriptionManager := graphqlws.NewSubscriptionManager()

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

for _, conn := range subscriptions {
  if conn.User == "Joe" {
    for _, subscription := range subscriptions[conn] {
      // Things you have access to here:
      subscription.ID            // The subscription ID (unique per conn)
      subscription.OperationName // The name of the subcription
      subscription.Query         // The subscription query/queries string
      subscription.Variables     // The subscription variables
      subscription.Document      // The GraphQL AST for the subscription
      subscription.Fields        // The names of top-level queries
      subscription.Connection    // The GraphQL WS connection

      // Send query results back to the subscriber at any point
      subscription.SendData(subscription, &graphqlws.DataMessagePayload{
        Data: ...,   // Can be anything (interface{})
        Errors: ..., // Optional ([]error)
      })
    }
  }
```

## License

Copyright (C) 2017 Functional Foundry, LLC.

Licensed under the [MIT License](LICENSE.md).

[graphql over websocket protocol]: https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md
