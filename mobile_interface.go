package panthalassa

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	api "github.com/Bit-Nation/panthalassa/api"
	apiPB "github.com/Bit-Nation/panthalassa/api/pb"
	backend "github.com/Bit-Nation/panthalassa/backend"
	chat "github.com/Bit-Nation/panthalassa/chat"
	dapp "github.com/Bit-Nation/panthalassa/dapp"
	dAppReg "github.com/Bit-Nation/panthalassa/dapp/registry"
	db "github.com/Bit-Nation/panthalassa/db"
	keyManager "github.com/Bit-Nation/panthalassa/keyManager"
	p2p "github.com/Bit-Nation/panthalassa/p2p"
	profile "github.com/Bit-Nation/panthalassa/profile"
	queue "github.com/Bit-Nation/panthalassa/queue"
	uiapi "github.com/Bit-Nation/panthalassa/uiapi"
	bolt "github.com/coreos/bbolt"
	proto "github.com/golang/protobuf/proto"
	log "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
)

var panthalassaInstance *Panthalassa
var logger = log.Logger("panthalassa")

type UpStream interface {
	Send(data string)
}

type StartConfig struct {
	EncryptedKeyManager string `json:"encrypted_key_manager"`
	SignedProfile       string `json:"signed_profile"`
	EthWsEndpoint       string `json:"eth_ws_endpoint"`
	EnableDebugging     bool   `json:"enable_debugging"`
	PrivChatEndpoint    string `json:"private_chat_endpoint"`
	PrivChatBearerToken string `json:"private_chat_bearer_token"`
}

// create a new panthalassa instance
func start(dbDir string, km *keyManager.KeyManager, config StartConfig, client, uiUpstream UpStream) error {

	if config.EnableDebugging {
		log.SetDebugLogging()
	}

	//Exit if instance was already created and not stopped
	if panthalassaInstance != nil {
		return errors.New("call stop first in order to create a new panthalassa instance")
	}

	// device api
	deviceApi := api.New(client)

	// create p2p network
	p2pNetwork, err := p2p.New()
	if err != nil {
		return err
	}

	// open database
	dbPath, err := db.KMToDBPath(dbDir, km)
	if err != nil {
		return err
	}
	dbInstance, err := db.Open(dbPath, 0644, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return err
	}

	// create signed pre key storage
	signedPreKeyStorage := db.NewBoltSignedPreKeyStorage(dbInstance, km)

	// create backend
	trans := backend.NewWSTransport(config.PrivChatEndpoint, config.PrivChatBearerToken, km)

	backend, err := backend.NewBackend(trans, km, signedPreKeyStorage)
	if err != nil {
		return err
	}

	// ui api
	uiApi := uiapi.New(uiUpstream)

	// open message storage
	messageStorage := db.NewChatMessageStorage(dbInstance, []func(db.MessagePersistedEvent){}, km)

	// queue instance
	jobStorage := queue.NewStorage(dbInstance)
	q := queue.New(jobStorage, 250, 4)

	// chat
	chatInstance, err := chat.NewChat(chat.Config{
		MessageDB:            messageStorage,
		Backend:              backend,
		SharedSecretDB:       db.NewBoltSharedSecretStorage(dbInstance, km),
		KM:                   km,
		DRKeyStorage:         db.NewBoltDRKeyStorage(dbInstance, km),
		SignedPreKeyStorage:  signedPreKeyStorage,
		OneTimePreKeyStorage: db.NewBoltOneTimePreKeyStorage(dbInstance, km),
		UserStorage:          db.NewBoltUserStorage(dbInstance),
		UiApi:                uiApi,
		Queue:                q,
	})
	if err != nil {
		return err
	}

	// dApp storage
	dAppStorage := dapp.NewDAppStorage(dbInstance, uiApi)

	// dApp registry
	dAppRegistry, err := dAppReg.NewDAppRegistry(p2pNetwork.Host, dAppReg.Config{
		EthWSEndpoint: config.EthWsEndpoint,
	}, deviceApi, km, dAppStorage, messageStorage, dbInstance)
	if err != nil {
		return err
	}

	//Create panthalassa instance
	panthalassaInstance = &Panthalassa{
		km:          km,
		upStream:    client,
		api:         deviceApi,
		p2p:         p2pNetwork,
		dAppReg:     dAppRegistry,
		chat:        chatInstance,
		db:          dbInstance,
		dAppStorage: dAppStorage,
	}

	return nil
}

// start panthalassa
func Start(dbDir, config, password string, client, uiUpstream UpStream) error {

	// unmarshal config
	var c StartConfig
	if err := json.Unmarshal([]byte(config), &c); err != nil {
		return err
	}

	store, err := keyManager.UnmarshalStore([]byte(c.EncryptedKeyManager))
	if err != nil {
		return err
	}

	// open key manager with password
	km, err := keyManager.OpenWithPassword(store, password)
	if err != nil {
		return err
	}

	return start(dbDir, km, c, client, uiUpstream)
}

// create a new panthalassa instance with the mnemonic
func StartFromMnemonic(dbDir, config, mnemonic string, client, uiUpstream UpStream) error {

	// unmarshal config
	var c StartConfig
	if err := json.Unmarshal([]byte(config), &c); err != nil {
		return err
	}

	store, err := keyManager.UnmarshalStore([]byte(c.EncryptedKeyManager))
	if err != nil {
		return err
	}

	// create key manager
	km, err := keyManager.OpenWithMnemonic(store, mnemonic)
	if err != nil {
		return err
	}

	// create panthalassa instance
	return start(dbDir, km, c, client, uiUpstream)

}

//Eth Private key
func EthPrivateKey() (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	return panthalassaInstance.km.GetEthereumPrivateKey()

}

func EthAddress() (string, error) {
	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	return panthalassaInstance.km.GetEthereumAddress()
}

func SendResponse(id string, data string, responseError string, timeout int) error {

	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa")
	}

	dataBytes, decodingError := base64.StdEncoding.DecodeString(data)
	if decodingError != nil {
		return decodingError
	}

	resp := &apiPB.Response{}
	if err := proto.Unmarshal(dataBytes, resp); err != nil {
		return err
	}

	var err error
	if responseError != "" {
		err = errors.New(responseError)
	}

	return panthalassaInstance.api.Respond(id, resp, err, time.Duration(timeout)*time.Second)
}

//Export the current account store with given password
func ExportAccountStore(pw, pwConfirm string) (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	return panthalassaInstance.Export(pw, pwConfirm)

}

func IdentityPublicKey() (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	return panthalassaInstance.km.IdentityPublicKey()
}

func GetMnemonic() (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	return panthalassaInstance.km.GetMnemonic().String(), nil
}

func SignProfile(name, location, image string) (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa")
	}

	// sign profile
	p, err := profile.SignProfile(name, location, image, *panthalassaInstance.km)
	if err != nil {
		return "", err
	}

	// export profile to protobuf
	pp, err := p.ToProtobuf()
	if err != nil {
		return "", err
	}

	// marshal protobuf profile
	rawProfile, err := proto.Marshal(pp)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(rawProfile), nil

}

//Stop panthalassa
func Stop() error {

	//Exit if not started
	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa in order to stop it")
	}

	//Stop panthalassa
	err := panthalassaInstance.Stop()
	if err != nil {
		//Reset singleton
		panthalassaInstance = nil
		return err
	}

	//Reset singleton
	panthalassaInstance = nil

	return nil
}

// fetch the identity public key of the
func GetIdentityPublicKey() (string, error) {

	//Exit if not started
	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa first")
	}

	return panthalassaInstance.km.IdentityPublicKey()

}

// connect the host to DApp development server
func ConnectToDAppDevHost(address string) error {

	//Exit if not started
	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa first")
	}

	maAddr, err := ma.NewMultiaddr(address)
	if err != nil {
		return err
	}

	return panthalassaInstance.dAppReg.ConnectDevelopmentServer(maAddr)

}

func OpenDApp(id, context string) error {

	//Exit if not started
	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa first")
	}

	// decode public key
	dAppSigningKey, err := hex.DecodeString(id)
	if err != nil {
		return err
	}
	if len(dAppSigningKey) != 32 {
		return errors.New("invalid DApp signing key")
	}

	return panthalassaInstance.dAppReg.OpenDApp(dAppSigningKey, context)

}

func StartDApp(dAppSingingKeyStr string, timeout int) error {

	//Exit if not started
	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa first")
	}

	// decode singing key
	dAppSigningKey, err := hex.DecodeString(dAppSingingKeyStr)
	if err != nil {
		return err
	}

	// signing key must be 32 bytes long since it's an ed25519 pub key
	if len(dAppSigningKey) != 32 {
		return errors.New("DApp singing key must be 32 bytes long")
	}

	return panthalassaInstance.dAppReg.StartDApp(dAppSigningKey, time.Second*time.Duration(timeout))

}

func RenderMessage(signingKey, payload string) (string, error) {

	//Exit if not started
	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa first")
	}

	// decode signing key
	dAppSigningKey, err := hex.DecodeString(signingKey)
	if err != nil {
		return "", err
	}
	if len(dAppSigningKey) != 32 {
		return "", errors.New("dapp signign key must be 32 bytes long")
	}

	return panthalassaInstance.dAppReg.RenderMessage(dAppSigningKey, payload)

}

func CallDAppFunction(signingKey string, id int, args string) error {

	// make sure we get an uint value
	if id < 0 {
		return errors.New("got negative number but need uint")
	}

	//Exit if not started
	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa first")
	}

	// decode signing key
	dAppSigningKey, err := hex.DecodeString(signingKey)
	if err != nil {
		return err
	}
	if len(dAppSigningKey) != 32 {
		return errors.New("dapp signign key must be 32 bytes long")
	}

	return panthalassaInstance.dAppReg.CallFunction(dAppSigningKey, uint(id), args)

}

func StopDApp(dAppSingingKeyStr string) error {

	if panthalassaInstance == nil {
		return errors.New("you have to start panthalassa first")
	}

	// decode singing key
	dAppSigningKey, err := hex.DecodeString(dAppSingingKeyStr)
	if err != nil {
		return err
	}

	// signing key must be 32 bytes long since it's and ed25519 pub key
	if len(dAppSigningKey) != 32 {
		return errors.New("DApp singing key must be 32 bytes long")
	}

	return panthalassaInstance.dAppReg.ShutDown(dAppSigningKey)

}

func DApps() (string, error) {

	if panthalassaInstance == nil {
		return "", errors.New("you have to start panthalassa first")
	}

	// fetch dApps
	dApps, err := panthalassaInstance.dAppStorage.All()
	if err != nil {
		return "", err
	}

	// marshal dapps
	rawDApps, err := json.Marshal(dApps)
	return string(rawDApps), err

}
