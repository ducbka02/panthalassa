package panthalassa

import (
	api "github.com/Bit-Nation/panthalassa/api/device"
	deviceApi "github.com/Bit-Nation/panthalassa/api/device"
	keyManager "github.com/Bit-Nation/panthalassa/keyManager"
)

type panthalassa struct {
	km        *keyManager.KeyManager
	upStream  api.UpStream
	deviceApi *deviceApi.Api
}

//Get ethereum private key of panthalassa instance
func (p panthalassa) EthereumPrivateKey() (string, error) {
	return p.km.GetEthereumPrivateKey()
}

//Stop the panthalassa instance
//this becomes interesting when we start
//to use the mesh network
func (p *panthalassa) Stop() error {
	return nil
}

//Export account with the given password
func (p *panthalassa) Export(pw, pwConfirm string) (string, error) {
	return p.km.Export(pw, pwConfirm)
}