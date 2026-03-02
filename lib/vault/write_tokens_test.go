package vault

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

func TestEncryptionMethodFromName(t *testing.T) {
	v := &Vault{}
	id := security.NewPrivateIDMust().PublicIDMust()

	enc, ecID, err := v.encryptionMethodFromName("hello.txt")
	core.TestErr(t, err, "unexpected parse error: %v")
	core.Assert(t, enc == "aes", "expected default aes mode")
	core.Assert(t, ecID == "", "expected empty ec recipient")

	enc, ecID, err = v.encryptionMethodFromName("hello.txt,aes")
	core.TestErr(t, err, "unexpected parse error: %v")
	core.Assert(t, enc == "aes", "expected aes mode")
	core.Assert(t, ecID == "", "expected empty ec recipient")

	enc, ecID, err = v.encryptionMethodFromName("hello.txt,public")
	core.TestErr(t, err, "unexpected parse error: %v")
	core.Assert(t, enc == "public", "expected public mode")
	core.Assert(t, ecID == "", "expected empty ec recipient")

	enc, ecID, err = v.encryptionMethodFromName("hello.txt,ec=" + id.String())
	core.TestErr(t, err, "unexpected parse error: %v")
	core.Assert(t, enc == "ec", "expected ec mode")
	core.Assert(t, ecID == id, "expected ec recipient to match")

	_, _, err = v.encryptionMethodFromName("hello.txt,ec=")
	core.Assert(t, err != nil, "expected empty ec recipient to fail")
}
