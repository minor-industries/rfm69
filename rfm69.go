package rfm69

import (
	"fmt"
	"github.com/pkg/errors"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"time"
)

type Board interface {
	TxSPI(w, r []byte) error
	Reset(bool) error
}

func Run(board Board) error {
	if err := board.Reset(true); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(300 * time.Millisecond) // TODO: shorten to optimal value

	if err := board.Reset(false); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(300 * time.Millisecond) // TODO: shorten to optimal value

	{
		t0 := time.Now()
		for {
			a := mustReadReg(board, REG_SYNCVALUE1)
			fmt.Printf("val = 0x%02x\n", a)
			if a == 0xAA {
				break
			}
			mustWriteReg(board, REG_SYNCVALUE1, 0xAA)
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	{
		t0 := time.Now()
		for {
			a := mustReadReg(board, REG_SYNCVALUE1)
			fmt.Printf("val = 0x%02x\n", a)
			if a == 0x55 {
				break
			}
			mustWriteReg(board, REG_SYNCVALUE1, 0x55)
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	intr := gpioreg.ByName("GPIO24")
	err := intr.In(gpio.Float, gpio.RisingEdge)
	noErr(errors.Wrap(err, "in"))

	go func() {
		for {
			intr.WaitForEdge(-1)
			fmt.Println("edge")
		}

	}()

	setConfig(board, getConfig(RF69_433MHZ, 100))
	setHighPower(board)
	sendFrame(board, 2, 1, []byte("abc123\x00"))

	select {}

	return nil
}

func setHighPower(board Board) {
	mustWriteReg(board, REG_TESTPA1, 0x5D)
	mustWriteReg(board, REG_TESTPA2, 0x7C)

	mustWriteReg(board, REG_OCP, RF_OCP_OFF)
	//enable P1 & P2 amplifier stages
	mustWriteReg(
		board,
		REG_PALEVEL,
		(mustReadReg(board, REG_PALEVEL)&0x1F)|RF_PALEVEL_PA1_ON|RF_PALEVEL_PA2_ON,
	)
}

func sendFrame(board Board, toAddr byte, fromAddr byte, msg []byte) {
	mustWriteReg(
		board,
		REG_OPMODE,
		mustReadReg(board, REG_OPMODE)&0xE3|RF_OPMODE_STANDBY,
	)

	for {
		val := mustReadReg(board, REG_IRQFLAGS1)
		if val&RF_IRQFLAGS1_MODEREADY == 0x00 {
			continue
		}
		break
	}

	fmt.Println("here1")
	mustWriteReg(board, REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00)

	ack := byte(0)

	tx := []byte{
		REG_FIFO | 0x80,
		byte(len(msg) + 3),
		toAddr,
		fromAddr,
		ack,
	}

	tx = append(tx, msg...)

	err := board.TxSPI(
		tx,
		nil,
	)
	noErr(errors.Wrap(err, "tx"))

	mustWriteReg(
		board,
		REG_OPMODE,
		mustReadReg(board, REG_OPMODE)&0xE3|RF_OPMODE_TRANSMITTER,
		// high power???
	)

	for {
		val := mustReadReg(board, REG_IRQFLAGS2)
		if val&RF_IRQFLAGS2_PACKETSENT == 0x00 {
			continue
		}
		break
	}
	fmt.Println("here2")
}

func setConfig(board Board, config [][2]byte) {
	for _, kv := range config {
		fmt.Printf("config 0x%02x = 0x%02x\n", kv[0], kv[1])
		mustWriteReg(board, kv[0], kv[1])
	}
}

func noErr(err error) {
	if err != nil {
		panic(err)
	}
}

func mustWriteReg(board Board, addr byte, value byte) {
	rx := make([]byte, 2)

	if err := board.TxSPI(
		[]byte{addr | 0x80, value},
		rx,
	); err != nil {
		panic(errors.Wrap(err, "tx"))
	}
}

func mustReadReg(board Board, addr byte) byte {
	rx := make([]byte, 2)

	if err := board.TxSPI(
		[]byte{addr & 0x7F, 0},
		rx,
	); err != nil {
		panic(errors.Wrap(err, "tx"))
	}

	return rx[1]
}
