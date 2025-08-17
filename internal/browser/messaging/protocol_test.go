package messaging

import (
	"bytes"
	"encoding/json"
	"testing"

	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
)

func Test_USMMux(t *testing.T) {
	t.Run("not registered action", func(t *testing.T) {
		req := &Request{Action: 100}
		mux := NewUSMMux(&ListVaultHandler{})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		var eia *ActionNotRegisteredError
		assert.ErrorAs(t, err, &eia)
	})

	t.Run("registered action", func(t *testing.T) {
		req := &Request{Action: ListVaultAction}
		s := &usm.StorageMock{}
		s.OnVaults = func() ([]string, error) {
			return []string{}, nil
		}
		mux := NewUSMMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
	})

	t.Run("list vaults", func(t *testing.T) {
		vaults := []string{"vault1", "vault2"}
		s := &usm.StorageMock{}
		s.OnVaults = func() ([]string, error) {
			return vaults, nil
		}

		req := &Request{Action: ListVaultAction}
		mux := NewUSMMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
		assert.NotEmpty(t, out)

		res, err := readNativeMessage(out)
		assert.Nil(t, err)
		v := &ListVaultHandlerResponsePayload{}
		err = json.Unmarshal(res.Payload, v)
		assert.Nil(t, err)
		assert.Equal(t, vaults, v.Vaults)
	})
}
