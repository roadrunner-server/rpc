package rpc

type API struct {
	cfg     []byte
	version string
}

func (a *API) Config(_ *bool, out *[]byte) error {
	*out = a.cfg
	return nil
}

func (a *API) Version(_ *bool, out *string) error {
	*out = a.version
	return nil
}
