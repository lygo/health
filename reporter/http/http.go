package http

import (
	"net/http"

	"bytes"
	"fmt"

	"encoding/json"

	"github.com/lygo/health"
)

type Registrator interface {
	Handle(pattern string, handler http.Handler)
}

// read that docs for understend more params
// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/
func WrapHandler(livenessPath string, readinessPath string, healthPath string, healther health.Healther, router Registrator) {
	if healthPath == `` {
		healthPath = `/health`
	}
	if livenessPath == `` {
		livenessPath = `/liveness`
	}

	if readinessPath == `` {
		readinessPath = `/readiness`
	}

	// for kube - check transport
	router.Handle(livenessPath, LivenessHandler())
	// for kube - ability to provide services
	router.Handle(readinessPath, ReadinessHandler(healther))
	// for show full info
	router.Handle(healthPath, HealthHandler(healther))
}

func LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
}

func ReadinessHandler(healther health.Healther) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := healther.Check(r.Context())
		if state.Status == health.HealthServing {
			w.Write([]byte(`OK`))
		} else {
			buff := bytes.NewBuffer(nil)
			for _, component := range state.Components {
				switch component.Status {
				case health.HealthComponentStatusFail:
					buff.WriteString(fmt.Sprintf(
						"- %s %s: %s",
						component.ComponentName,
						component.Status.String(),
						component.Description,
					))
				case health.HealthComponentStatusTimeout:
					buff.WriteString(fmt.Sprintf(
						"- %s %s: %s %s",
						component.ComponentName,
						component.Status.String(),
						component.Description,
						component.Duration.String(),
					))
				}
			}

			http.Error(w, buff.String(), http.StatusServiceUnavailable)
		}
	})
}

func HealthHandler(healther health.Healther) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		state := healther.Check(r.Context())

		if err := json.NewEncoder(w).Encode(state); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
