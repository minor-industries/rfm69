package rfm69

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"time"
)

type Board interface {
	TxSPI(w, r []byte) error
	Reset(bool) error
	WaitForD0Edge()
}

type Radio struct {
	board Board
	log   func(string)
}

func NewRadio(board Board, log func(string)) *Radio {
	return &Radio{board: board, log: log}
}

func (r *Radio) Setup() error {
	if err := r.board.Reset(true); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(100 * time.Microsecond)

	if err := r.board.Reset(false); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(5 * time.Millisecond)

	{
		t0 := time.Now()
		for {
			a, err := r.readRegReturningErrors(REG_SYNCVALUE1)
			if err != nil {
				return errors.Wrap(err, "read syncvalue1")
			}
			r.log(fmt.Sprintf("val = 0x%02x", a))
			if a == 0xAA {
				break
			}
			if err := r.writeRegReturningErrors(REG_SYNCVALUE1, 0xAA); err != nil {
				return errors.Wrap(err, "write syncvalue1")
			}
			if time.Now().Sub(t0) > 15*time.Second {
				return errors.New("not syncing")
			}
		}
	}

	{
		t0 := time.Now()
		for {
			a, err := r.readRegReturningErrors(REG_SYNCVALUE1)
			if err != nil {
				return errors.Wrap(err, "read syncvalue1")
			}
			r.log(fmt.Sprintf("val = 0x%02x", a))
			if a == 0x55 {
				break
			}
			if err := r.writeRegReturningErrors(REG_SYNCVALUE1, 0x55); err != nil {
				return errors.Wrap(err, "write syncvalue1")
			}
			if time.Now().Sub(t0) > 15*time.Second {
				return errors.New("not syncing")
			}
		}
	}

	if err := r.setConfig(
		getConfig(RF69_433MHZ, 100),
	); err != nil {
		return errors.Wrap(err, "set config")
	}

	if err := r.setHighPower(); err != nil {
		return errors.Wrap(err, "set high power")
	}

	return nil
}

func (r *Radio) Tx() error {
	go func() {
		for {
			r.board.WaitForD0Edge()
			r.log(fmt.Sprintf("edge"))
		}

	}()

	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		if err := r.SendFrame(
			2,
			1,
			[]byte("abc123\x00"),
		); err != nil {
			return errors.Wrap(err, "send frame")
		}
	}

	return nil
}

func (r *Radio) Rx() error {
	intrCh := make(chan struct{})
	errCh := make(chan error)

	go func() {
		for {
			r.board.WaitForD0Edge()
			intrCh <- struct{}{}
		}
	}()

	go func() {
		for {
			err := r.beginReceive()
			if err != nil {
				errCh <- errors.Wrap(err, "begin receive")
				return
			}

			<-intrCh
			r.log("got interrupt")

			tx := []byte{REG_FIFO & 0x7f, 0, 0, 0, 0}
			rx := make([]byte, len(tx))

			if err := r.board.TxSPI(
				tx,
				rx,
			); err != nil {
				errCh <- errors.Wrap(err, "txspi")
				return
			}

			rx = rx[1:]
			r.log("rx: " + hex.Dump(rx))

			payloadLength := rx[0]
			targetID := rx[1]
			senderID := rx[2]
			ctlByte := rx[3]

			r.log(fmt.Sprintf(
				"len=%d, target=0x%02x, sender=0x%02x, ctl=0x%02x",
				payloadLength,
				targetID,
				senderID,
				ctlByte,
			))

			dataLength := payloadLength - 3

			{
				tx := []byte{REG_FIFO & 0x7f}
				tx = append(tx, bytes.Repeat([]byte{0}, int(dataLength))...)
				rx := make([]byte, len(tx))
				if err := r.board.TxSPI(tx, rx); err != nil {
					errCh <- errors.Wrap(err, "spi")
					return
				}
				rx = rx[1:]
				r.log("data: " + hex.Dump(rx))
			}
		}
	}()

	return errors.Wrap(<-errCh, "error channel")
}

func (r *Radio) beginReceive() error {
	irqflags2, err := r.readRegReturningErrors(REG_IRQFLAGS2)
	if err != nil {
		return err
	}

	if irqflags2&RF_IRQFLAGS2_PAYLOADREADY != 0 {
		// avoid RX deadlocks??
		r.editReg(REG_PACKETCONFIG2, func(val byte) byte {
			return val&0xFB | RF_PACKET2_RXRESTART
		})
	}

	if err := r.writeRegReturningErrors(REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_01); err != nil {
		return err
	}

	r.editReg(REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_RECEIVER
	})

	// set low power regs
	if err := r.writeRegReturningErrors(REG_TESTPA1, 0x55); err != nil {
		return err
	}
	if err := r.writeRegReturningErrors(REG_TESTPA2, 0x70); err != nil {
		return err
	}
	return nil
}

func (r *Radio) setHighPower() error {
	if err := r.writeRegReturningErrors(REG_TESTPA1, 0x5D); err != nil {
		return err
	}
	if err := r.writeRegReturningErrors(REG_TESTPA2, 0x7C); err != nil {
		return err
	}

	if err := r.writeRegReturningErrors(REG_OCP, RF_OCP_OFF); err != nil {
		return err
	}

	//enable P1 & P2 amplifier stages
	r.editReg(REG_PALEVEL, func(val byte) byte {
		return val&0x1F | RF_PALEVEL_PA1_ON | RF_PALEVEL_PA2_ON
	})

	return nil
}

func (r *Radio) SendFrame(toAddr byte, fromAddr byte, msg []byte) error {
	r.editReg(REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_STANDBY
	})

	for {
		val, err := r.readRegReturningErrors(REG_IRQFLAGS1)
		if err != nil {
			return errors.Wrap(err, "read")
		}
		if val&RF_IRQFLAGS1_MODEREADY == 0x00 {
			continue
		}
		break
	}

	r.log(fmt.Sprintf("here1"))
	if err := r.writeRegReturningErrors(REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00); err != nil {
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

	if err := r.board.TxSPI(
		tx,
		nil,
	); err != nil {
		return errors.Wrap(err, "tx spi")
	}

	r.editReg(REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_TRANSMITTER
	})

	for {
		val, err := r.readRegReturningErrors(REG_IRQFLAGS2)
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

func (r *Radio) setConfig(config [][2]byte) error {
	for _, kv := range config {
		r.log(fmt.Sprintf("config 0x%02x = 0x%02x", kv[0], kv[1]))
		if err := r.writeRegReturningErrors(kv[0], kv[1]); err != nil {
			return errors.Wrap(err, "write")
		}
	}

	return nil
}

func (r *Radio) readRegReturningErrors(addr byte) (byte, error) {
	rx := make([]byte, 2)

	if err := r.board.TxSPI(
		[]byte{addr & 0x7F, 0},
		rx,
	); err != nil {
		return 0, errors.Wrap(err, "tx")
	}

	return rx[1], nil
}

func (r *Radio) readReg(addr byte) byte {
	val, _ := r.readRegReturningErrors(addr)
	return val
}

func (r *Radio) writeRegReturningErrors(addr byte, value byte) error {
	rx := make([]byte, 2)

	if err := r.board.TxSPI(
		[]byte{addr | 0x80, value},
		rx,
	); err != nil {
		return errors.Wrap(err, "tx")
	}
	return nil
}

func (r *Radio) writeReg(addr byte, value byte) {
	_ = r.writeRegReturningErrors(addr, value)
}

func (r *Radio) editReg(
	addr byte,
	edit func(val byte) byte,
) {
	val := r.readReg(addr)
	newVal := edit(val)
	r.writeReg(addr, newVal)
}
