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

func (r *Radio) sync(val byte) error {
	for i := 0; i < 15; i++ {
		a, err := r.readRegReturningErrors(REG_SYNCVALUE1)
		if err != nil {
			return errors.Wrap(err, "read syncvalue1")
		}
		r.log(fmt.Sprintf("val = 0x%02x", a))
		if a == val {
			return nil
		}
		if err := r.writeRegReturningErrors(REG_SYNCVALUE1, val); err != nil {
			return errors.Wrap(err, "write syncvalue1")
		}
	}
	return errors.New("radio is not syncing")
}

func (r *Radio) Setup(freq byte) error {
	if err := r.board.Reset(true); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(100 * time.Microsecond)

	if err := r.board.Reset(false); err != nil {
		return errors.Wrap(err, "reset")
	}

	time.Sleep(5 * time.Millisecond)

	if err := r.sync(0xAA); err != nil {
		return errors.Wrap(err, "sync 1")
	}

	if err := r.sync(0x55); err != nil {
		return errors.Wrap(err, "sync 2")
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

type Packet struct {
	Src     byte
	Dst     byte
	RSSI    int
	Payload []byte
}

func (r *Radio) Rx(out chan<- *Packet) error {
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

			rssi := r.readRSSI()
			r.log(fmt.Sprintf("rssi = %d", rssi))

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

				out <- &Packet{
					Src:     senderID,
					Dst:     targetID,
					RSSI:    rssi,
					Payload: rx,
				}
			}
		}
	}()

	return errors.Wrap(<-errCh, "error channel")
}

func (r *Radio) beginReceive() error {
	if r.readReg(REG_IRQFLAGS2)&RF_IRQFLAGS2_PAYLOADREADY != 0 {
		// avoid RX deadlocks??
		r.editReg(REG_PACKETCONFIG2, func(val byte) byte {
			return val&0xFB | RF_PACKET2_RXRESTART
		})
	}

	r.writeReg(REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_01)
	r.editReg(REG_OPMODE, func(val byte) byte {
		return val&0xE3 | RF_OPMODE_RECEIVER
	})

	// set low power regs
	r.writeReg(REG_TESTPA1, 0x55)
	r.writeReg(REG_TESTPA2, 0x70)

	return nil
}

func (r *Radio) setHighPower() error {
	r.writeReg(REG_TESTPA1, 0x5D)
	r.writeReg(REG_TESTPA2, 0x7C)

	r.writeReg(REG_OCP, RF_OCP_OFF)

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
		if r.readReg(REG_IRQFLAGS1)&RF_IRQFLAGS1_MODEREADY == 0x00 {
			continue
		}
		break
	}

	r.log(fmt.Sprintf("here1"))
	r.writeReg(REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00)

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
		if r.readReg(REG_IRQFLAGS2)&RF_IRQFLAGS2_PACKETSENT == 0x00 {
			continue
		}
		break
	}

	return nil
}

func (r *Radio) setConfig(config [][2]byte) error {
	for _, kv := range config {
		r.log(fmt.Sprintf("config 0x%02x = 0x%02x", kv[0], kv[1]))
		r.writeReg(kv[0], kv[1])
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

func (r *Radio) readRSSI() int {
	val := int(r.readReg(REG_RSSIVALUE)) * -1
	return val / 2
}
