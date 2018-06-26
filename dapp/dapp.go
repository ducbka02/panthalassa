package dapp

import (
	"encoding/hex"
	"fmt"

	module "github.com/Bit-Nation/panthalassa/dapp/module"
	dAppRenderer "github.com/Bit-Nation/panthalassa/dapp/module/renderer/dapp"
	msgRenderer "github.com/Bit-Nation/panthalassa/dapp/module/renderer/message"
	logger "github.com/op/go-logging"
	otto "github.com/robertkrimen/otto"
)

type DApp struct {
	vm           *otto.Otto
	logger       *logger.Logger
	app          *JsonRepresentation
	closeChan    chan<- *JsonRepresentation
	dAppRenderer *dAppRenderer.Module
	msgRenderer  *msgRenderer.Module
}

// close DApp
func (d *DApp) Close() {
	d.vm.Interrupt <- func() {
		d.logger.Info(fmt.Sprintf("shutting down: %s (%s)", hex.EncodeToString(d.app.SignaturePublicKey), d.app.Name))
		d.closeChan <- d.app
	}
}

func (d *DApp) ID() string {
	return hex.EncodeToString(d.app.SignaturePublicKey)
}

func (d *DApp) RenderDApp(context string) error {
	return d.dAppRenderer.OpenDApp(context)
}

func (d *DApp) RenderMessage(msg, context string) (string, error) {
	return d.msgRenderer.RenderMessage(msg, context)
}

// will start a DApp based on the given config file
//
func New(l *logger.Logger, app *JsonRepresentation, vmModules []module.Module, closer chan<- *JsonRepresentation) (*DApp, error) {

	// check if app is valid
	valid, err := app.VerifySignature()
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, InvalidSignature
	}

	// create VM
	vm := otto.New()
	vm.Interrupt = make(chan func(), 1)

	// register all vm modules
	for _, m := range vmModules {
		if err := m.Register(vm); err != nil {
			return nil, err
		}
	}

	// register DApp renderer
	dr := dAppRenderer.New(l)
	if err := dr.Register(vm); err != nil {
		return nil, err
	}
	
	// register message renderer
	mr := msgRenderer.New(l)
	if err := dr.Register(vm); err != nil {
		return nil, err
	}

	dApp := &DApp{
		vm:           vm,
		logger:       l,
		app:          app,
		closeChan:    closer,
		dAppRenderer: dr,
		msgRenderer:  mr,
	}

	go func() {
		_, err := vm.Run(app.Code)
		if err != nil {
			l.Error(err.Error())
			closer <- app
		}
	}()

	return dApp, nil
}
