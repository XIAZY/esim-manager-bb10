// eSIM Manager manages the profiles on a removable eUICC from a BlackBerry 10
// phone. The eUICC logic is Go (internal/euicc, internal/es9p, internal/app);
// C is limited to the modem's SATSA library (internal/satsa) and the Cascades
// UI bridge (internal/ui).
package main

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"time"

	"esimmanager/internal/app"
	"esimmanager/internal/euicc"
	"esimmanager/internal/satsa"
	"esimmanager/internal/ui"
)

// The UI runs on the main goroutine, kept on the process's main thread.
func init() { runtime.LockOSThread() }

func main() {
	a := app.New(ui.Publisher{}, func() (app.Card, error) {
		c, err := satsa.Open(euicc.ISDR)
		if err != nil {
			return nil, err
		}
		return c, nil
	})
	go a.Run()

	rc := ui.Run(func(name, argsJSON string) {
		var args map[string]string
		if argsJSON != "" {
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				log.Printf("ui: bad arguments for %s: %v", name, err)
				return
			}
		}
		a.Send(app.Command{Name: name, Args: args})
	})
	// Close the SIM channel before exiting (the modem keeps it otherwise), then
	// drop our access grant, which should exist only while the app runs.
	a.Stop(5 * time.Second)
	if err := satsa.RevokeAccess(); err != nil {
		log.Printf("revoking SIM access: %v", err)
	}
	os.Exit(rc)
}
