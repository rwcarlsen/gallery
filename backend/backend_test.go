package backend

import (
	"bytes"
	"testing"
)

// Note that test order helps failed test feedback more meaningful

func TestMake(t *testing.T) {
	if _, err := Make(dummy, Params{}); err != nil {
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
		Btype:   dummy,
		Bparams: Params{},
	}

	if _, err := s.Make(); err != nil {
		t.Error(err)
	}
}

func TestSpecList_GetSet(t *testing.T) {
	s := SpecList{}
	if spec := s.Get("foo1"); spec != nil {
		t.Fatal("Get did not return nil for non-existent spec.")
	}

	s.Set("foo1", &Spec{Btype: Amazon, Bparams: Params{"Name": "foo1name"}})
	if spec := s.Get("foo1"); spec == nil {
		t.Fatal("Get return nil for existing spec.")
	} else if nm := spec.Bparams["Name"]; nm != "foo1name" {
		t.Errorf("spec name: expected foo1name, got %v", nm)
	}

	s.Set("foo1", &Spec{Btype: Amazon, Bparams: Params{"Name": "foo2name"}})
	if nm := s.Get("foo1").Bparams["Name"]; nm != "foo2name" {
		t.Error("Spec was not overwritten with duplicate name usage")
	}
}

func TestSpecList_SaveLoad(t *testing.T) {
	s := SpecList{}
	s.Set("foo1", &Spec{Btype: Amazon, Bparams: Params{"Name": "foo1name"}})
	s.Set("foo2", &Spec{Btype: Local, Bparams: Params{"Name": "foo2name"}})
	var w bytes.Buffer
	if err := s.Save(&w); err != nil {
		t.Fatal(err)
	}

	list, err := LoadSpecList(&w)
	if err != nil {
		t.Fatal(err)
	}

	if tp := list.Get("foo1").Btype; tp != Amazon {
		t.Errorf("foo1's type (%v) is not %v", tp, Amazon)
	}
	if list.Get("foo1").Bparams["Name"] != "foo1name" {
		t.Error("foo1's Name param is not foo1name")
	}

	if tp := list.Get("foo2").Btype; tp != Local {
		t.Errorf("foo2's type (%v) is not %v", tp, Local)
	}
	if list.Get("foo2").Bparams["Name"] != "foo2name" {
		t.Error("foo2's Name param is not foo2name")
	}
}

func TestSpecList_LoadMake(t *testing.T) {
	buf := bytes.NewBufferString(testSpecList)
	set, err := LoadSpecList(buf)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := set.Make("dummy"); err != nil {
		t.Error(err)
	}
	if _, err := set.Make("hd"); err != nil {
		t.Error(err)
	}
	if _, err := set.Make("amz"); err != nil {
		t.Error(err)
	}
}

const testSpecList = `
{
  "dummy": {
	"Btype": "dummy",
	"Bparams": {
	  "Name": "dummy1"
	}
  },
  "amz": {
	"Btype": "Amazon-S3",
	"Bparams": {
	  "Name": "amz",
	  "AccessKeyId": "AAAAAAAAAAAAAAAAAAAA",
	  "SecretAccessKey": "7777777777777777777777777777777777777777"
	}
  },
  "hd": {
	"Btype": "Local-HD",
	"Bparams": {
	  "Name": "hd",
	  "Root": "/media/spare"
	}
  }
}`
