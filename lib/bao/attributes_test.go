package bao

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func TestAttributes(t *testing.T) {
	db := sqlx.NewTestDB(t, "vault.db", "")
	alice := security.NewPrivateIDMust()

	tmpFolder := t.TempDir()
	storeConfig := storage.StoreConfig{
		Id:   "local-test-store",
		Type: "local",
		Local: storage.LocalConfig{
			Base: "file://" + tmpFolder,
		},
	}

	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "cannot create vault")
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
