
package backend

import (
	"bytes"
	"testing"
)

func TestRegister(t *testing.T) {
}

func TestMake(t *testing.T) {
	if _, err := Make(dummy, Params{}); err != nil {
		t.Error(err)
	}
}

func TestSpec(t *testing.T) {
	s := &Spec{
		Btype: dummy,
		Bparams: Params{},
	}

	if _, err := s.Make(); err != nil {
		t.Error(err)
	}
}

func TestSpecList_LoadSave(t *testing.T) {
}

func TestSpecList_GetSet(t *testing.T) {
}

func TestSpecList_Make(t *testing.T) {
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
