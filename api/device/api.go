package device_api

import (
	"encoding/json"
	"errors"

	"fmt"
	"github.com/Bit-Nation/panthalassa/api/device/rpc"
	log "github.com/ipfs/go-log"
	valid "gopkg.in/asaskevich/govalidator.v4"
)

var logger = log.Logger("device_api")

type UpStream interface {
	//Send data to client
	Send(data string)
}

type apiCall struct {
	Type string `json:"type"`
	Id   uint32 `json:"id"`
	Data string `json:"data"`
}

func (c *apiCall) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

type rawResponse struct {
	Error   string `json:"error",valid:"string,optional"`
	Payload string `json:"payload",valid:"string,optional"`
}

type Response struct {
	Error   error
	Payload string
	Closer  chan error
}

func (r *Response) Close(err error) {
	r.Closer <- err
}

type Api struct {
	device UpStream
	state  *State
}

func New(deviceInterface UpStream) *Api {

	api := Api{
		state:  newState(),
		device: deviceInterface,
	}

	return &api
}

//Send a call to the api
func (a *Api) Send(call rpc.JsonRPCCall) (<-chan Response, error) {

	//Validate call
	if err := call.Valid(); err != nil {
		return nil, err
	}

	//Get call data
	callContent, err := call.Data()
	if err != nil {
		return nil, err
	}

	//Create internal representation
	c := apiCall{
		Type: call.Type(),
		Data: callContent,
	}
	respChan := make(chan Response, 1)
	c.Id = a.state.Add(respChan)

	//Marshal the call data
	callData, err := c.Marshal()
	if err != nil {
		return nil, err
	}

	//Send json rpc call to device
	go a.device.Send(string(callData))

	return respChan, nil

}

// @todo at the moment the fetched response channel will never close in case there we return earlier with an error
func (a *Api) Receive(id uint32, data string) error {

	logger.Debug(fmt.Sprintf("Got response for request (%d) - with data: %s", id, data))

	// get the response channel
	resp, err := a.state.Cut(id)
	if err != nil {
		return err
	}

	// closer
	closer := make(chan error)

	// decode raw response
	var rr rawResponse
	if err := json.Unmarshal([]byte(data), &rr); err != nil {
		return err
	}

	// validate raw response
	_, err = valid.ValidateStruct(rr)
	if err != nil {
		return err
	}

	// construct response
	r := Response{
		Error:   err,
		Payload: rr.Payload,
		Closer:  closer,
	}
	if rr.Error != "" {
		r.Error = errors.New(rr.Error)
	}

	// send response to response channel
	resp <- r

	logger.Debug("send response", r)

	return <-closer

}
