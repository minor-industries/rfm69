package main

import (
	"fmt"
	"github.com/pkg/errors"
	"log"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
	"time"
)

const (
	REG_SYNCVALUE1 = 0x2F
)

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

	fmt.Println(conn.String())

	t0 := time.Now()

	for a := mustReadReg(conn, REG_SYNCVALUE1); a != 0xAA; {
		fmt.Printf("val = 0x%02x\n", a)
		mustWriteReg(conn, REG_SYNCVALUE1, 0xAA)
		if time.Now().Sub(t0) > 15*time.Second {
			panic("not syncing")
		}
	}

	return nil
}

func mustWriteReg(conn spi.Conn, addr byte, value byte) {
	rx := make([]byte, 2)

	if err := conn.Tx(
		[]byte{addr | 0x80, value},
		rx,
	); err != nil {
		panic(errors.Wrap(err, "tx"))
	}
}

func mustReadReg(conn spi.Conn, addr byte) byte {
	rx := make([]byte, 2)

	if err := conn.Tx(
		[]byte{addr & 0x7F, 0},
		rx,
	); err != nil {
		panic(errors.Wrap(err, "tx"))
	}

	return rx[0]
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
