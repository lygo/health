package health

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

/*
  service can be have other component, but in hole we have two type of components
   - requered
   - optional
 each component must implement Health interface
 also each component must register to Global Healther
*/

type HealthComponentStatus int

const (
	HealthComponentStatusUnknown HealthComponentStatus = iota
	// if configuration is OK and component have all dependens for work and base checked will be done
	// component will return HealthStatusOn
	HealthComponentStatusOn
	// if component disable by default or by configuration - component on check return HealthStatusOff
	HealthComponentStatusOff
	// if component enable on config and something gose wron, return HealthStatusFail and return description
	// error can be return by
	// - config - value of config is BAD
	// - deps - some address not availability or file not exists or something else more
	// - condition - if you component have some smoke test running of start
	HealthComponentStatusFail

	HealthComponentStatusTimeout
)

func (h HealthComponentStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + h.String() + `"`), nil
}

var healthComponentsStatus = map[HealthComponentStatus]string{
	HealthComponentStatusUnknown: `unknown`,
	HealthComponentStatusOn:      `on`,
	HealthComponentStatusOff:     `off`,
	HealthComponentStatusFail:    `fail`,
	HealthComponentStatusTimeout: `timeout`,
}

func (h HealthComponentStatus) String() string {
	str, ok := healthComponentsStatus[h]
	if !ok {
		return `unknown`
	}

	return str
}

type HealthStatus int

const (
	HealthUnknown HealthStatus = iota
	HealthServing
	HealthNotServing
)

var healthStatuses = []string{
	"UNKNOWN",
	"SERVING",
	"NOT_SERVING",
}

func (h HealthStatus) MarshalJSON() ([]byte, error) {
	var msg string
	if h < 0 || h > 2 {
		msg = healthStatuses[0]
	} else {
		msg = healthStatuses[h]
	}
	return []byte(`"` + msg + `"`), nil
}

type HealthComponentState struct {
	Status HealthComponentStatus `json:"status"`
	// description of status
	Description string `json:"description"`

	Stats map[string]int `json:"stats"`
}

type Health struct {
	Status HealthStatus `json:"status"`

	// stats by status
	Stats map[string]int `json:"stats"`

	Components []HealthComponentDescription `json:"components"`

	Duration time.Duration `json:"duration"`
}

type ComponentHealther interface {
	Check(ctx context.Context) HealthComponentState
}

type componentPresent int

const (
	ComponentPresentRequired componentPresent = iota
	ComponentPresentOptional
)

type componentOption struct {
	Type componentPresent
}

type Option func(*componentOption)

func SetOptional() Option {
	return func(o *componentOption) {
		o.Type = ComponentPresentOptional
	}
}

func (h *healther) Register(componentName string, health ComponentHealther, opts ...Option) error {
	h.regLock.Lock()
	defer h.regLock.Unlock()

	if _, ok := h.healthers[componentName]; ok {
		return fmt.Errorf("component %s already registered", componentName)
	}
	h.healthers[componentName] = health

	comOpt := new(componentOption)
	for _, opt := range opts {
		opt(comOpt)
	}

	h.componentOptions[componentName] = comOpt

	return nil
}

func (h *healther) UnRegister(componentName string) {
	h.regLock.Lock()
	defer h.regLock.Unlock()

	delete(h.healthers, componentName)
	delete(h.componentOptions, componentName)
}

type HealthComponentDescription struct {
	ComponentName string `json:"component_name"`

	Status HealthComponentStatus `json:"status"`
	// description of status
	Description string `json:"description"`

	Stats map[string]int `json:"stats,omitempty"`

	// needed time for check finally status
	Duration time.Duration `json:"duration"`
}

func wrapHealthCheck(ctx context.Context, start time.Time, name string, healther ComponentHealther) HealthComponentDescription {

	state := healther.Check(ctx)
	// if status not OK, please repeat and wait
	if state.Status == HealthComponentStatusFail {
		select {
		case <-ctx.Done():
			return HealthComponentDescription{
				ComponentName: name,
				Status:        HealthComponentStatusTimeout,
				Description:   ctx.Err().Error(),
				Duration:      time.Since(start),
			}
		default:
			// TODO: add optinos and timeout and repeater
			return wrapHealthCheck(ctx, start, name, healther)
		}
	}

	return HealthComponentDescription{
		ComponentName: name,
		Status:        state.Status,
		Description:   state.Description,
		Stats:         state.Stats,
		Duration:      time.Since(start),
	}
}

func (h *healther) gatheringStateOfHealth(ctx context.Context) <-chan HealthComponentDescription {

	var (
		start = time.Now().UTC()
		out   = make(chan HealthComponentDescription, 0)
		wg    = new(sync.WaitGroup)
	)

	h.regLock.RLock()
	defer h.regLock.RUnlock()

	for componentName, check := range h.healthers {
		wg.Add(1)

		go func(name string, healther ComponentHealther) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				out <- HealthComponentDescription{
					ComponentName: name,
					Status:        HealthComponentStatusTimeout,
					Description:   ctx.Err().Error(),
					Duration:      time.Since(start),
				}
				return
			case out <- wrapHealthCheck(ctx, start, name, healther):
			}
		}(componentName, check)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

type Healther interface {
	// Check - gathering state of health registred components and summe result
	Check(ctx context.Context) Health

	Register(componentName string, health ComponentHealther, opts ...Option) error

	UnRegister(componentName string)
}

func New() Healther {
	return &healther{
		regLock:          new(sync.RWMutex),
		healthers:        make(map[string]ComponentHealther, 0),
		componentOptions: make(map[string]*componentOption, 0),
	}
}

type healther struct {
	regLock          *sync.RWMutex
	healthers        map[string]ComponentHealther
	componentOptions map[string]*componentOption
}

// Check - gathering state of health registred components and summe result
func (h *healther) Check(ctx context.Context) Health {
	var (
		start  = time.Now().UTC()
		result = Health{
			Status:     HealthUnknown,
			Stats:      make(map[string]int, 0),
			Components: make([]HealthComponentDescription, 0),
		}
	)

	for healthOfComponent := range h.gatheringStateOfHealth(ctx) {
		opt := h.componentOptions[healthOfComponent.ComponentName]

		result.Stats[healthOfComponent.Status.String()]++

		switch healthOfComponent.Status {
		case HealthComponentStatusOff,
			HealthComponentStatusFail,
			HealthComponentStatusTimeout,
			HealthComponentStatusUnknown:
			if opt.Type == ComponentPresentRequired {
				result.Status = HealthNotServing
			}
		case HealthComponentStatusOn:
			if result.Status == HealthUnknown {
				result.Status = HealthServing
			}
		default:
			if opt.Type == ComponentPresentRequired {
				result.Status = HealthNotServing
			}
		}

		result.Components = append(result.Components, healthOfComponent)
	}

	result.Duration = time.Since(start)

	if result.Status == HealthUnknown {
		if _, ok := result.Stats[HealthComponentStatusOn.String()]; !ok {
			result.Status = HealthNotServing
		}
	}

	// sort by latency  of component states
	sort.Slice(result.Components, func(i, j int) bool {
		return result.Components[i].Duration < result.Components[j].Duration
	})

	return result
}
