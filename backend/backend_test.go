package backend

import (
	"testing"
)

func TestMake(t *testing.T) {
	Register("dummy", func(p Params) (Interface, error) { return nil, nil })
	if _, err := Make("dummy", Params{}); err != nil {
		t.Error(err)
	}
}

func TestRegister(t *testing.T) {
	if _, err := Make("floopy", Params{}); err == nil {
		t.Error("Expected error for Make non-existing type, got nil.")
	}
	Register("floopy", func(p Params) (Interface, error) { return nil, nil })
	if _, err := Make("floopy", Params{}); err != nil {
		t.Errorf("Make for newly registered type failed: %v", err)
	}
}

func TestSpec(t *testing.T) {
	s := &Spec{
		Type:   "dummy",
		Params: Params{},
	}

	if _, err := s.Make(); err != nil {
		t.Error(err)
	}
}
