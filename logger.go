package graphqlws

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

// NewLogger returns a beautiful logger that logs messages with a
// given prefix (typically the name of a system component / subsystem).
func NewLogger(prefix string) *log.Entry {
	logger := log.New()
	logger.Formatter = new(prefixed.TextFormatter)
	logger.Level = log.GetLevel()
	return logger.WithField("prefix", fmt.Sprintf("graphqlws/%s", prefix))
}
