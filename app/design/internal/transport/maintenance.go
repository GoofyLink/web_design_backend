package transport

import (
	"context"
	"log"
	"time"
)

func (a *API) Maintenance(ctx context.Context) error {
	if e := a.Operations.Cleanup(ctx); e != nil {
		return e
	}
	n, e := a.Workspace.Cleanup(ctx)
	if e != nil {
		return e
	}
	files, e := a.Assets.Cleanup(ctx)
	if e == nil {
		log.Printf("maintenance: expired designs=%d reclaimed files=%d", n, files)
	}
	return e
}
func (a *API) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run, cancel := context.WithTimeout(ctx, 45*time.Second)
			if e := a.Maintenance(run); e != nil {
				log.Printf("maintenance: %v", e)
			}
			cancel()
		}
	}
}
