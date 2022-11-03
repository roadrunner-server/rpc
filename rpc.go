package rpc

type API struct {
	cfg []byte
}

func (a *API) Config(_ *bool, out *[]byte) error {
	*out = a.cfg
	return nil
}
