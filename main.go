package main

import (
	"fmt"
	"github.com/pkg/errors"
	"log"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
	"time"
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

	rst := gpioreg.ByName("GPIO5")

	err = rst.Out(gpio.High)
	noErr(errors.Wrap(err, "high"))

	time.Sleep(300 * time.Millisecond)

	err = rst.Out(gpio.Low)
	noErr(errors.Wrap(err, "low"))

	time.Sleep(300 * time.Millisecond)

	{
		t0 := time.Now()
		for {
			a := mustReadReg(conn, REG_SYNCVALUE1)
			fmt.Printf("val = 0x%02x\n", a)
			if a == 0xAA {
				break
			}
			mustWriteReg(conn, REG_SYNCVALUE1, 0xAA)
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	{
		t0 := time.Now()
		for {
			a := mustReadReg(conn, REG_SYNCVALUE1)
			fmt.Printf("val = 0x%02x\n", a)
			if a == 0x55 {
				break
			}
			mustWriteReg(conn, REG_SYNCVALUE1, 0x55)
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	setConfig(conn, getConfig(RF69_433MHZ, 100))
	sendFrame(conn, 2, 1, []byte("abc123"))

	return nil
}

func sendFrame(
	conn spi.Conn,
	toAddr byte,
	fromAddr byte,
	msg []byte,
) {
	mustWriteReg(
		conn,
		REG_OPMODE,
		mustReadReg(conn, REG_OPMODE)&0xE3|RF_OPMODE_STANDBY,
	)

	for {
		val := mustReadReg(conn, REG_IRQFLAGS1)
		if val&RF_IRQFLAGS1_MODEREADY == 0x00 {
			fmt.Println("continue1")
			continue
		}
		break
	}

	fmt.Println("here1")
	mustWriteReg(conn, REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00)

	ack := byte(0)

	tx := []byte{
		REG_FIFO | 0x80,
		byte(len(msg) + 3),
		toAddr,
		fromAddr,
		ack,
	}

	tx = append(tx, msg...)

	err := conn.Tx(
		tx,
		nil,
	)
	noErr(errors.Wrap(err, "tx"))

	mustWriteReg(
		conn,
		REG_OPMODE,
		mustReadReg(conn, REG_OPMODE)&0xE3|RF_OPMODE_TRANSMITTER,
		// high power???
	)

	for {
		val := mustReadReg(conn, REG_IRQFLAGS2)
		if val&RF_IRQFLAGS2_PACKETSENT == 0x00 {
			fmt.Println("continue2")
			continue
		}
		break
	}
	fmt.Println("here2")
}

func setConfig(conn spi.Conn, config [][2]byte) {
	for _, kv := range config {
		fmt.Printf("config 0x%02x = 0x%02x\n", kv[0], kv[1])
		mustWriteReg(conn, kv[0], kv[1])
	}
}

func noErr(err error) {
	if err != nil {
		panic(err)
	}
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

	return rx[1]
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
