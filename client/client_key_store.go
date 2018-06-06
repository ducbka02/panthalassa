package client

import (
	"encoding/hex"

	deviceApi "github.com/Bit-Nation/panthalassa/api/device"
	keyManager "github.com/Bit-Nation/panthalassa/keyManager"
	log "github.com/ipfs/go-log"
	dr "github.com/tiabc/doubleratchet"
)

var logger = log.Logger("client - double ratchet key")

type DoubleRatchetKeyStore struct {
	api *deviceApi.Api
	km  *keyManager.KeyManager
}

// get a key by it's key and msg number
func (s *DoubleRatchetKeyStore) Get(k dr.Key, msgNum uint) (mk dr.Key, ok bool) {

	respCha, err := s.api.Send(&DRKeyStoreGetCall{
		Key:    hex.EncodeToString(k[:]),
		MsgNum: msgNum,
	})

	if err != nil {
		return dr.Key{}, false
	}

	resp := <-respCha
	if resp.Error != nil {
		resp.Close(nil)
		return dr.Key{}, false
	}

	keyResp, err := UnmarshalDRKeyStoreGetResponse(resp.Payload, s.km)
	if err != nil {
		resp.Close(err)
		return keyResp.Key, false
	}

	resp.Close(nil)
	return keyResp.Key, keyResp.Ok

}

// save message key (double ratchet key)
func (s *DoubleRatchetKeyStore) Put(k dr.Key, msgNum uint, mk dr.Key) {

	// encrypt message key
	ct, err := s.km.AESEncrypt(mk[:])
	if err != nil {
		logger.Error(err)
	}

	// send request to device api
	respChan, err := s.api.Send(&DRKeyStorePutCall{
		IndexKey:         hex.EncodeToString(k[:]),
		MsgNumber:        msgNum,
		DoubleRatchetKey: ct,
	})
	if err != nil {
		logger.Error(err)
		return
	}

	// wait for response and close it since we don't need it somewhere else
	resp := <-respChan
	resp.Close(nil)

	if resp.Error != nil {
		logger.Error(resp.Error)
	}

}

func (s *DoubleRatchetKeyStore) DeleteMk(k dr.Key, msgNum uint) {

	respCha, err := s.api.Send(&DRKeyStoreDeleteMK{
		IndexKey:  hex.EncodeToString(k[:]),
		MsgNumber: msgNum,
	})
	if err != nil {
		logger.Error(err)
		return
	}

	resp := <-respCha
	resp.Close(nil)

	if resp.Error != nil {
		logger.Error(resp.Error)
	}

}

func (s *DoubleRatchetKeyStore) DeletePk(k dr.Key) {

}

func (s *DoubleRatchetKeyStore) Count(k dr.Key) uint {
	return 9
}

func (s *DoubleRatchetKeyStore) All() map[dr.Key]map[uint]dr.Key {
	return map[dr.Key]map[uint]dr.Key{}
}

func New() *DoubleRatchetKeyStore {
	return &DoubleRatchetKeyStore{}
}
