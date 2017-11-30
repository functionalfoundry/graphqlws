# graphqlws

Implementation of the Apollo GraphQL WebSocket protocol in Go.

## Getting started

1. Install dependencies:
   ```sh
   go get github.com/sirupsen/logrus
   go get github.com/x-cray/logrus-prefixed-formatter
   go get github.com/google/uuid
   go get github.com/gorilla/websocket
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

## License

Copyright (C) 2017 Functional Foundry, LLC.

Licensed under the [MIT License](LICENSE.md).
