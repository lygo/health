package health

import (
	"testing"
	"time"
	"context"
	"encoding/json"
)

type mockComponent struct {
	timeout time.Duration
	countFail int
	status HealthComponentStatus
	desc string
}

func (m *mockComponent) Check(ctx context.Context) HealthComponentState {
	time.Sleep(m.timeout)

	if m.countFail > 0 {
		m.countFail--
		return HealthComponentState{
			Status:HealthComponentStatusFail,
			Description:"server not found",
		}
	}


	return HealthComponentState{
		Status:m.status,
		Description:m.desc,
	}
}


func TestSuccessHealth(t *testing.T) {
	healtherObj := New()
	healtherObj.Register(`componentA`,&mockComponent{
		timeout: time.Millisecond * 500,
		status: HealthComponentStatusOn,
		desc: `start work on port 934534`,
	})
	healtherObj.Register(`componentB`,&mockComponent{
		timeout: time.Microsecond * 500,
		status: HealthComponentStatusOff,
		desc: `dns not set - skip`,
	},SetOptional())

	healtherObj.Register(`componentC`,&mockComponent{
		timeout: time.Microsecond * 500,
		countFail: 5,
		status: HealthComponentStatusOn,
		desc: `success connect Yo!`,
	})

	ctx,cancel := context.WithTimeout(context.Background(),time.Second)
	defer cancel()

	state := healtherObj.Check(ctx)


	if state.Status != HealthServing {
		t.Errorf("%s != %s",HealthServing,state.Status)
	}

	if ons := state.Stats[HealthComponentStatusOn.String()];ons != 2 {
		t.Errorf("%d != %d",2,ons)
	}

	if offs := state.Stats[HealthComponentStatusOff.String()];offs != 1 {
		t.Errorf("%d != %d",1,offs)
	}
}


func TestFailWithRequiredHealth(t *testing.T) {
	healtherObj := New()
	healtherObj.Register(`componentA`,&mockComponent{
		timeout: time.Millisecond * 500,
		status: HealthComponentStatusOn,
		desc: `start work on port 934534`,
	})

	healtherObj.Register(`componentB`,&mockComponent{
		timeout: time.Microsecond * 500,
		status: HealthComponentStatusOff,
		desc: `dns not set - skip`,
	})

	healtherObj.Register(`componentC`,&mockComponent{
		timeout: time.Microsecond * 500,
		countFail: 5,
		status: HealthComponentStatusOn,
		desc: `success connect Yo!`,
	})

	ctx,cancel := context.WithTimeout(context.Background(),time.Second)
	defer cancel()

	state := healtherObj.Check(ctx)


	if state.Status != HealthNotServing {
		t.Errorf("%s != %s",HealthNotServing,state.Status)
	}

	if ons := state.Stats[HealthComponentStatusOn.String()];ons != 2 {
		t.Errorf("%d != %d",2,ons)
	}

	if offs := state.Stats[HealthComponentStatusOff.String()];offs != 1 {
		t.Errorf("%d != %d",1,offs)
	}
}

func TestFailWithCtxHealth(t *testing.T) {
	healtherObj := New()
	healtherObj.Register(`componentA`,&mockComponent{
		timeout: time.Millisecond * 500,
		countFail:5,
		status: HealthComponentStatusOn,
		desc: `start work on port 934534`,
	})

	healtherObj.Register(`componentB`,&mockComponent{
		timeout: time.Microsecond * 500,
		status: HealthComponentStatusOff,
		desc: `dns not set - skip`,
	})

	healtherObj.Register(`componentC`,&mockComponent{
		timeout: time.Microsecond * 500,
		status: HealthComponentStatusOn,
		desc: `success connect Yo!`,
	})

	ctx,cancel := context.WithTimeout(context.Background(),time.Millisecond * 800)
	defer cancel()

	state := healtherObj.Check(ctx)

	data, _ := json.MarshalIndent(state,"","\t")
	t.Log(string(data))

	if state.Status != HealthNotServing {
		t.Errorf("%s != %s",HealthNotServing,state.Status)
	}

	if ons := state.Stats[HealthComponentStatusOn.String()];ons != 1 {
		t.Errorf("stats.%s %d != %d",HealthComponentStatusOn.String(),1,ons)
	}

	if offs := state.Stats[HealthComponentStatusOff.String()];offs != 1 {
		t.Errorf("stats.%s %d != %d",HealthComponentStatusOff.String(),1,offs)
	}

	if v := state.Stats[HealthComponentStatusFail.String()];v != 0 {
		t.Errorf("stats.%s %d != %d",HealthComponentStatusFail.String(),0,v)
	}

	if v := state.Stats[HealthComponentStatusTimeout.String()];v != 1 {
		t.Errorf("stats.%s %d != %d",HealthComponentStatusTimeout.String(),1,v)
	}
}
