package main

import (
	"net/http"

	"github.com/functionalfoundry/graphqlws/server"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)
	log.Info("Starting example server on :8085")

	subscriptionManager := graphqlws.NewSubscriptionManager()
	websocketHandler := graphqlws.NewHandler(subscriptionManager)

	http.Handle("/subscriptions", websocketHandler)

	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.WithField("error", err).Error("Failed to start server")
	}
}
