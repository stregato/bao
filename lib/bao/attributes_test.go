package bao

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func TestAttributes(t *testing.T) {
	db := sqlx.NewTestDB(t, "stash.db", "")
	alice := security.NewPrivateIDMust()

	tmpFolder := t.TempDir()
	storeUrl := "file://" + tmpFolder
	s, err := Create(db, alice, storeUrl, Config{})
	core.TestErr(t, err, "cannot create stash")
	defer s.Close()

	err = s.SetAttribute(0, "attr1", "value1")
	core.TestErr(t, err, "cannot set attribute attr1")
	err = s.SetAttribute(0, "attr2", "value2")
	core.TestErr(t, err, "cannot set attribute attr2")

	val, err := s.GetAttribute("attr1", alice.PublicIDMust())
	core.TestErr(t, err, "cannot get attribute attr1")
	core.Assert(t, val == "value1", "expected value1, got %s", val)
	val, err = s.GetAttribute("attr2", alice.PublicIDMust())
	core.TestErr(t, err, "cannot get attribute attr2")
	core.Assert(t, val == "value2", "expected value2, got %s", val)

	attrs, err := s.GetAttributes(alice.PublicIDMust())
	core.TestErr(t, err, "cannot get attributes")
	core.Assert(t, len(attrs) == 2, "expected 2 attributes, got %d", len(attrs))
	core.Assert(t, attrs["attr1"] == "value1", "expected attr1 to be value1, got %s", attrs["attr1"])
	core.Assert(t, attrs["attr2"] == "value2", "expected attr2 to be value2, got %s", attrs["attr2"])
}
