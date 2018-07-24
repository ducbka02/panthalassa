package backend

import (
	"errors"
	"fmt"
	"sync"
	"time"

	km "github.com/Bit-Nation/panthalassa/keyManager"
	bpb "github.com/Bit-Nation/protobuffers"
	log "github.com/ipfs/go-log"
)

var logger = log.Logger("backend")

// IMPORTANT - the returned error will be send to the backend.
// Make sure it only return an error message that doesn't
// have private information
type RequestHandler func(req *bpb.BackendMessage_Request) (*bpb.BackendMessage_Response, error)

type ServerConfig struct {
	WebSocketUrl string
	BearerToken  string
}

type Backend struct {
	transport Transport
	// all outgoing requests
	outReqQueue    chan *request
	lock           sync.Mutex
	stack          requestStack
	requestHandler []RequestHandler
	km             *km.KeyManager
	authenticated  bool
	closer         chan struct{}
}

// Add request handler that will be executed
func (b *Backend) AddRequestHandler(handler RequestHandler) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.requestHandler = append(b.requestHandler, handler)
}

func (b *Backend) Start() error {
	return b.transport.Start()
}

func (b *Backend) Close() error {
	b.closer <- struct{}{}
	return b.transport.Close()
}

func NewServerBackend(trans Transport, km *km.KeyManager) (*Backend, error) {

	b := &Backend{
		transport:   trans,
		outReqQueue: make(chan *request, 150),
		lock:        sync.Mutex{},
		stack: requestStack{
			stack: map[string]chan *response{},
			lock:  sync.Mutex{},
		},
		requestHandler: []RequestHandler{},
		km:             km,
		closer:         make(chan struct{}, 1),
	}

	// handle incoming message and iterate over
	// the registered message handlers
	trans.OnMessage(func(msg *bpb.BackendMessage) error {
		b.lock.Lock()
		defer b.lock.Unlock()
		
		// make sure we don't get a response & a request at the same time
		// we don't accept it. It's invalid!
		if msg.Request != nil && msg.Response != nil {
			return errors.New("a message can’t have a response and a request at the same time")
		}
		
		// handle requests
		if msg.Request != nil {
			for _, handler := range b.requestHandler {
				// handler
				h := handler
				resp, err := h(msg.Request)
				// exit on error
				if err != nil {
					return b.transport.Send(&bpb.BackendMessage{
						RequestID: msg.RequestID,
						Error:     err.Error(),
					})
				}
				// if resp is nil we know that the handler didn't handle the request
				if resp == nil {
					continue
				}
				// send response
				err = b.transport.Send(&bpb.BackendMessage{
					Response:  resp,
					RequestID: msg.RequestID,
				})
				if err != nil {
					return err
				}

			}
		}

		// handle responses
		if msg.Response != nil {

			resp := msg.Response
			reqID := msg.RequestID

			// err will be != nil in the case of no response channel
			respChan := b.stack.Cut(reqID)
			if respChan == nil {
				return fmt.Errorf("failed to fetch response channel for id: %s", msg.RequestID)
			}

			// send error from response to request channel
			if msg.Error != "" {
				respChan <- &response{
					err: errors.New(msg.Error),
				}
				return nil
			}

			// in the case this was a auth request we need to apply some special logic
			// this will only be executed when this message was a auth request
			if resp.Auth != nil {
				b.authenticated = true
			}

			// send received response to response channel
			respChan <- &response{
				resp: resp,
			}

		}

		logger.Warning("dropping message: %s", msg)

		return nil
	})

	// auth request handler
	b.AddRequestHandler(b.auth)

	// send outgoing requests to transport
	go func() {
		for {

			// wait for authentication
			b.lock.Lock()
			if !b.authenticated {
				time.Sleep(time.Second * 1)
				b.lock.Unlock()
				continue
			}
			b.lock.Unlock()

			select {
			case <-b.closer:
				return
			case req := <-b.outReqQueue:
				// add response channel
				b.stack.Add(req.ReqID, req.RespChan)
				// send request
				go func() {
					err := b.transport.Send(&bpb.BackendMessage{
						RequestID: req.ReqID,
						Request:   req.Req,
					})
					// close response channel on error
					if err != nil {
						req.RespChan <- &response{
							err: err,
						}
					}
				}()
			}
		}
	}()

	return b, nil

}