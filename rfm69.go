package rfm69

import (
	"fmt"
	"github.com/pkg/errors"
	"time"
)

type Board interface {
	TxSPI(w, r []byte) error
	Reset(bool) error
	WaitForD0Edge()
}

func Run(
	board Board,
	log func(string),
) error {
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
			a, err := readReg(board, REG_SYNCVALUE1)
			if err != nil {
				return errors.Wrap(err, "read syncvalue1")
			}
			log(fmt.Sprintf("val = 0x%02x\n", a))
			if a == 0xAA {
				break
			}
			if err := writeReg(board, REG_SYNCVALUE1, 0xAA); err != nil {
				return errors.Wrap(err, "write syncvalue1")
			}
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	{
		t0 := time.Now()
		for {
			a, err := readReg(board, REG_SYNCVALUE1)
			if err != nil {
				return errors.Wrap(err, "read syncvalue1")
			}
			log(fmt.Sprintf("val = 0x%02x\n", a))
			if a == 0x55 {
				break
			}
			if err := writeReg(board, REG_SYNCVALUE1, 0x55); err != nil {
				return errors.Wrap(err, "write syncvalue1")
			}
			if time.Now().Sub(t0) > 15*time.Second {
				panic("not syncing")
			}
		}
	}

	if err := setConfig(
		board,
		log,
		getConfig(RF69_433MHZ, 100),
	); err != nil {
		return errors.Wrap(err, "set config")
	}

	if err := setHighPower(board); err != nil {
		return errors.Wrap(err, "set high power")
	}

	go func() {
		for {
			board.WaitForD0Edge()
			log(fmt.Sprintf("edge\n"))
		}

	}()

	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		if err := sendFrame(
			board,
			log,
			2,
			1,
			[]byte("abc123\x00"),
		); err != nil {
			return errors.Wrap(err, "send frame")
		}
	}

	return nil
}

func setHighPower(board Board) error {
	if err := writeReg(board, REG_TESTPA1, 0x5D); err != nil {
		return err
	}
	if err := writeReg(board, REG_TESTPA2, 0x7C); err != nil {
		return err
	}

	if err := writeReg(board, REG_OCP, RF_OCP_OFF); err != nil {
		return err
	}

	//enable P1 & P2 amplifier stages
	if err := editReg(board, REG_PALEVEL, func(val byte) byte {
		return val&0x1F | RF_PALEVEL_PA1_ON | RF_PALEVEL_PA2_ON
	}); err != nil {
		return errors.Wrap(err, "edit")
	}

	return nil
}

func sendFrame(board Board, log func(string), toAddr byte, fromAddr byte, msg []byte) error {
	if err := editReg(board, REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_STANDBY
	}); err != nil {
		return errors.Wrap(err, "edit")
	}

	for {
		val, err := readReg(board, REG_IRQFLAGS1)
		if err != nil {
			return errors.Wrap(err, "read")
		}
		if val&RF_IRQFLAGS1_MODEREADY == 0x00 {
			continue
		}
		break
	}

	log(fmt.Sprintf("here1\n"))
	if err := writeReg(board, REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00); err != nil {
		return errors.Wrap(err, "write")
	}

	ack := byte(0)

	tx := []byte{
		REG_FIFO | 0x80,
		byte(len(msg) + 3),
		toAddr,
		fromAddr,
		ack,
	}

	tx = append(tx, msg...)

	if err := board.TxSPI(
		tx,
		nil,
	); err != nil {
		return errors.Wrap(err, "tx spi")
	}

	if err := editReg(board, REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_TRANSMITTER
	}); err != nil {
		return errors.Wrap(err, "edit")
	}

	for {
		val, err := readReg(board, REG_IRQFLAGS2)
		if err != nil {
			return errors.Wrap(err, "read")
		}
		if val&RF_IRQFLAGS2_PACKETSENT == 0x00 {
			continue
		}
		break
	}

	return nil
}

func setConfig(board Board, log func(string), config [][2]byte) error {
	for _, kv := range config {
		log(fmt.Sprintf("config 0x%02x = 0x%02x\n", kv[0], kv[1]))
		if err := writeReg(board, kv[0], kv[1]); err != nil {
			return errors.Wrap(err, "write")
		}
	}

	return nil
}

func noErr(err error) {
	if err != nil {
		panic(err)
	}
}

func readReg(board Board, addr byte) (byte, error) {
	rx := make([]byte, 2)

	if err := board.TxSPI(
		[]byte{addr & 0x7F, 0},
		rx,
	); err != nil {
		return 0, errors.Wrap(err, "tx")
	}

	return rx[1], nil
}

func writeReg(board Board, addr byte, value byte) error {
	rx := make([]byte, 2)

	if err := board.TxSPI(
		[]byte{addr | 0x80, value},
		rx,
	); err != nil {
		return errors.Wrap(err, "tx")
	}
	return nil
}

func editReg(
	board Board,
	addr byte,
	edit func(val byte) byte,
) error {
	val, err := readReg(board, addr)
	if err != nil {
		return errors.Wrap(err, "read")
	}

	newVal := edit(val)

	if err := writeReg(
		board,
		addr,
		newVal,
	); err != nil {
		return errors.Wrap(err, "write")
	}

	return nil
}
