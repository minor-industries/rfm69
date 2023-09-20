package main

import (
	"bytes"
	"encoding/binary"
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

type SensorData struct {
	Temperature      float32 // celsius
	RelativeHumidity float32
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

	log := func(s string) {
		fmt.Println(s)
	}
	radio := rfm69.NewRadio(
		&Board{spi: conn, rst: rst, intr: intr},
		log,
	)

	if err := radio.Setup(); err != nil {
		return errors.Wrap(err, "setup")
	}

	packets := make(chan *rfm69.Packet)

	go func() {
		for packet := range packets {
			log(fmt.Sprintf("got packet: %v", packet))
			msgType := packet.Payload[0]
			switch msgType {
			case 1:
				msg := &SensorData{}
				err := binary.Read(bytes.NewBuffer(packet.Payload[1:]), binary.LittleEndian, msg)
				if err != nil {
					log("error reading message: " + err.Error())
					break
				}
				log(fmt.Sprintf("msg = %v", msg))
				log(fmt.Sprintf("temp = %fF", (msg.Temperature*9/5)+32))
			default:
				log(fmt.Sprintf("unknown message type: %d", msgType))
			}
		}
	}()

	if err := radio.Rx(packets); err != nil {
		return errors.Wrap(err, "rx")
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
