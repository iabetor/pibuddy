package pipeline

import "testing"

func TestNewStateMachine_InitialStateIsIdle(t *testing.T) {
	sm := NewStateMachine()
	if sm.Current() != StateIdle {
		t.Fatalf("expected initial state Idle, got %s", sm.Current())
	}
}

func TestStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		from, to State
	}{
		{StateIdle, StateListening},
		{StateListening, StateProcessing},
		{StateProcessing, StateSpeaking},
		{StateSpeaking, StateIdle},
	}

	for _, tt := range tests {
		sm := NewStateMachine()
		// Advance to the 'from' state via valid transitions
		advanceTo(t, sm, tt.from)

		if !sm.Transition(tt.to) {
			t.Errorf("transition %s → %s should be valid", tt.from, tt.to)
		}
		if sm.Current() != tt.to {
			t.Errorf("expected state %s, got %s", tt.to, sm.Current())
		}
	}
}

func TestStateMachine_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from, to State
	}{
		{StateIdle, StateProcessing},
		{StateIdle, StateSpeaking},
		{StateListening, StateSpeaking},
		{StateListening, StateListening},
		{StateProcessing, StateListening},
		{StateProcessing, StateProcessing},
		{StateSpeaking, StateListening},
		{StateSpeaking, StateProcessing},
		{StateSpeaking, StateSpeaking},
	}

	for _, tt := range tests {
		sm := NewStateMachine()
		advanceTo(t, sm, tt.from)

		if sm.Transition(tt.to) {
			t.Errorf("transition %s → %s should be invalid", tt.from, tt.to)
		}
		if sm.Current() != tt.from {
			t.Errorf("state should remain %s after invalid transition, got %s", tt.from, sm.Current())
		}
	}
}

func TestStateMachine_AnyStateToIdle(t *testing.T) {
	states := []State{StateIdle, StateListening, StateProcessing, StateSpeaking}

	for _, s := range states {
		sm := NewStateMachine()
		advanceTo(t, sm, s)

		if !sm.Transition(StateIdle) {
			t.Errorf("transition %s → Idle should always be valid", s)
		}
		if sm.Current() != StateIdle {
			t.Errorf("expected Idle, got %s", sm.Current())
		}
	}
}

func TestStateMachine_ForceIdle(t *testing.T) {
	states := []State{StateIdle, StateListening, StateProcessing, StateSpeaking}

	for _, s := range states {
		sm := NewStateMachine()
		advanceTo(t, sm, s)

		sm.ForceIdle()
		if sm.Current() != StateIdle {
			t.Errorf("ForceIdle from %s: expected Idle, got %s", s, sm.Current())
		}
	}
}

func TestStateMachine_OnChangeCallback(t *testing.T) {
	sm := NewStateMachine()

	var calledFrom, calledTo State
	callCount := 0
	sm.SetOnChange(func(from, to State) {
		calledFrom = from
		calledTo = to
		callCount++
	})

	sm.Transition(StateListening)
	if callCount != 1 {
		t.Fatalf("expected onChange called once, got %d", callCount)
	}
	if calledFrom != StateIdle || calledTo != StateListening {
		t.Errorf("expected callback with Idle→Listening, got %s→%s", calledFrom, calledTo)
	}
}

func TestStateMachine_OnChangeNotCalledOnInvalid(t *testing.T) {
	sm := NewStateMachine()

	callCount := 0
	sm.SetOnChange(func(from, to State) {
		callCount++
	})

	sm.Transition(StateProcessing) // invalid from Idle
	if callCount != 0 {
		t.Errorf("expected onChange not called on invalid transition, got %d calls", callCount)
	}
}

func TestStateMachine_ForceIdleOnChangeCallback(t *testing.T) {
	sm := NewStateMachine()
	sm.Transition(StateListening)

	var calledFrom, calledTo State
	callCount := 0
	sm.SetOnChange(func(from, to State) {
		calledFrom = from
		calledTo = to
		callCount++
	})

	sm.ForceIdle()
	if callCount != 1 {
		t.Fatalf("expected onChange called once on ForceIdle, got %d", callCount)
	}
	if calledFrom != StateListening || calledTo != StateIdle {
		t.Errorf("expected Listening→Idle, got %s→%s", calledFrom, calledTo)
	}
}

func TestStateMachine_ForceIdleNoCallbackWhenAlreadyIdle(t *testing.T) {
	sm := NewStateMachine()

	callCount := 0
	sm.SetOnChange(func(from, to State) {
		callCount++
	})

	sm.ForceIdle()
	if callCount != 0 {
		t.Errorf("expected no onChange when ForceIdle from Idle, got %d calls", callCount)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		s    State
		want string
	}{
		{StateIdle, "Idle"},
		{StateListening, "Listening"},
		{StateProcessing, "Processing"},
		{StateSpeaking, "Speaking"},
		{State(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

// advanceTo transitions the state machine from Idle to the target state
// through valid intermediate transitions.
func advanceTo(t *testing.T, sm *StateMachine, target State) {
	t.Helper()
	path := []State{StateIdle, StateListening, StateProcessing, StateSpeaking}
	for _, s := range path {
		if s == target {
			return
		}
		idx := int(s) + 1
		if idx < len(path) {
			if !sm.Transition(path[idx]) {
				t.Fatalf("failed to advance to %s", path[idx])
			}
		}
	}
}
