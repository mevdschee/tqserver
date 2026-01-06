package supervisor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mevdschee/tqserver/internal/router"
)

// startHealthChecks monitors worker health with periodic HTTP checks
func (s *Supervisor) startHealthChecks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				workers := s.router.GetAllWorkers()
				for _, worker := range workers {
					if worker.Process == nil {
						continue
					}
					healthy := s.checkWorkerHealth(worker)
					worker.SetHealthy(healthy)
				}
			}
		}
	}()
}

// checkWorkerHealth performs an HTTP health check on a worker
func (s *Supervisor) checkWorkerHealth(worker *router.Worker) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", worker.Port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
