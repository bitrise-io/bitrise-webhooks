package metrics

import (
	"log"

	"github.com/yvasiyarov/gorelic"
)

var (
	newRelicAgent *gorelic.Agent
)

// SetupNewRelic ...
func SetupNewRelic(appName, newRelicLicenseKey string) {
	agent := gorelic.NewAgent()
	agent.NewrelicLicense = newRelicLicenseKey
	agent.NewrelicName = appName
	agent.CollectHTTPStat = true
	agent.Verbose = true

	if err := agent.Run(); err != nil {
		log.Printf(" [!] Exception: Failed to initialize NewRelic: %s", err)
	} else {
		log.Println(" (i) NewRelic setup [OK]")
		newRelicAgent = agent
	}
}
