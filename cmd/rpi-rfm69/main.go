package main

import (
	"fmt"
	"github.com/minor-industries/rfm69"
	"github.com/pkg/errors"
	"log"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

type Board struct {
	spi  spi.Conn
	rst  gpio.PinIO
	intr gpio.PinIO
}

func (b *Board) WaitForD0Edge() {
	b.intr.WaitForEdge(-1)
}

func (b *Board) TxSPI(w, r []byte) error {
	return b.spi.Tx(w, r)
}

func (b *Board) Reset(b2 bool) error {
	if b2 {
		return b.rst.Out(gpio.High)
	} else {
		return b.rst.Out(gpio.Low)
	}
}

func run() error {
	if _, err := host.Init(); err != nil {
		return errors.Wrap(err, "host init")
	}

	// Is this the right SPI bus??
	port, err := spireg.Open("")
	if err != nil {
		return errors.Wrap(err, "spireg open")
	}

	conn, err := port.Connect(4*physic.MegaHertz, spi.Mode3, 8)
	if err != nil {
		return errors.Wrap(err, "spi connect")
	}

	rst := gpioreg.ByName("GPIO5")

	intr := gpioreg.ByName("GPIO24")
	if err := intr.In(gpio.Float, gpio.RisingEdge); err != nil {
		return errors.Wrap(err, "gpio in")
	}

	board := &Board{spi: conn, rst: rst, intr: intr}

	log := func(s string) {
		fmt.Print(s)
	}

	return errors.Wrap(rfm69.Run(board, log), "run")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
