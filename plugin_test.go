package rpc

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

type jsonConfig struct {
	value string
}

func (c jsonConfig) RRVersion() string { return "test" }
func (c jsonConfig) Has(string) bool   { return true }
func (c jsonConfig) Unmarshal(out any) error {
	*out.(*any) = map[string]string{"value": c.value}
	return nil
}
func (c jsonConfig) UnmarshalKey(_ string, out any) error {
	*out.(*Config) = Config{Listen: "tcp://127.0.0.1:6001"}
	return nil
}
func (c jsonConfig) NamedLogger(string) *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestInitJSON(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid config", value: "value"},
		{name: "invalid utf8", value: "\xff", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := jsonConfig{value: tt.value}
			p := &Plugin{}
			err := p.Init(cfg, cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, `{"value":"value"}`, string(p.wcfg))
		})
	}
}
