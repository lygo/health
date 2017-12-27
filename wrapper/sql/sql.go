package sql

import (
	"context"
	gosql "database/sql"

	"github.com/lygo/health"
)

func New(db *gosql.DB) health.ComponentHealther {
	return &dbWrapper{
		db: db,
	}
}

type dbWrapper struct {
	db *gosql.DB
}

func (w *dbWrapper) Check(ctx context.Context) health.HealthComponentState {
	var (
		state health.HealthComponentState
	)

	if w.db == nil {
		state.Status = health.HealthComponentStatusOff
		state.Description = `component doesn't have *sql.DB`
		return state
	}

	err := w.db.PingContext(ctx)

	if err != nil {
		state.Status = health.HealthComponentStatusFail
		state.Description = err.Error()
	} else {
		state.Status = health.HealthComponentStatusOn
		state.Description = `connected`
	}

	state.Stats = map[string]int{
		"open_connections": w.db.Stats().OpenConnections,
	}

	return state
}
