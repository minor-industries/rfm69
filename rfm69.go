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
	board    Board
	log      func(string)
	fromAddr byte
	txPower  int
}

func NewRadio(
	board Board,
	log func(string),
	fromAddr byte,
	txPower int,
) *Radio {
	return &Radio{
		board:    board,
		log:      log,
		fromAddr: fromAddr,
		txPower:  txPower,
	}
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

	r.SetPowerDBm(13)

	if err := r.setConfig(
		getConfig(RF69_433MHZ, 100),
	); err != nil {
		return errors.Wrap(err, "set config")
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

type Mode int

const (
	ModeStandby Mode = iota + 1
	ModeTx      Mode = iota + 1
)

func (r *Radio) setMode(mode Mode) {
	switch mode {
	case ModeStandby:
		r.editReg(REG_OPMODE, func(val byte) byte {
			return val&0xE3 | RF_OPMODE_STANDBY
		})
	case ModeTx:
		r.editReg(REG_OPMODE, func(val byte) byte {
			return val&0xE3 | RF_OPMODE_TRANSMITTER
		})
	default:
		panic("unknown mode")
	}
}

func (r *Radio) waitForModeReady() {
	for r.readReg(REG_IRQFLAGS1)&RF_IRQFLAGS1_MODEREADY == 0x00 {
	}
}

func (r *Radio) SendFrame(
	toAddr byte,
	msg []byte,
) error {
	r.setMode(ModeStandby)
	r.waitForModeReady()
	r.clearFIFO()
	r.SetPowerDBm(r.txPower)
	r.writeReg(REG_DIOMAPPING1, RF_DIOMAPPING1_DIO0_00)

	ack := byte(0x00)

	tx := []byte{
		REG_FIFO | 0x80,
		byte(len(msg) + 3),
		toAddr,
		r.fromAddr,
		ack,
	}

	tx = append(tx, msg...)
	//tx = append(tx, 0)

	if err := r.board.TxSPI(
		tx,
		nil,
	); err != nil {
		return errors.Wrap(err, "tx spi")
	}

	r.setMode(ModeTx)
	r.waitForPacketSent()

	r.setMode(ModeStandby)
	r.waitForModeReady()
	r.SetPowerDBm(-2)

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

type levelSetting struct {
	paLevel   byte
	pa2       bool
	highPower bool
}

var powerLevelSettings = map[int]levelSetting{
	-2: {16, false, false},
	-1: {17, false, false},
	-0: {18, false, false},
	1:  {19, false, false},
	2:  {20, false, false},
	3:  {21, false, false},
	4:  {22, false, false},
	5:  {23, false, false},
	6:  {24, false, false},
	7:  {25, false, false},
	8:  {26, false, false},
	9:  {27, false, false},
	10: {28, false, false},
	11: {29, false, false},
	12: {30, false, false},
	13: {31, false, false},
	14: {28, true, false},
	15: {29, true, false},
	16: {30, true, false},
	17: {31, true, false},
	18: {29, true, true},
	19: {30, true, true},
	20: {31, true, true},
}

func (r *Radio) SetPowerDBm(val int) {
	val = max(val, -2)
	val = min(val, 20)

	settings := powerLevelSettings[val]

	if settings.highPower {
		r.writeReg(REG_OCP, RF_OCP_OFF)
		r.writeReg(REG_PALEVEL, settings.paLevel|RF_PALEVEL_PA1_ON|RF_PALEVEL_PA2_ON)
		r.writeReg(REG_TESTPA1, 0x5D)
		r.writeReg(REG_TESTPA2, 0x7C)
	} else if settings.pa2 {
		r.writeReg(REG_TESTPA1, 0x55)
		r.writeReg(REG_TESTPA2, 0x70)
		r.writeReg(REG_PALEVEL, settings.paLevel|RF_PALEVEL_PA1_ON|RF_PALEVEL_PA2_ON)
		r.writeReg(REG_OCP, RF_OCP_ON)
	} else {
		r.writeReg(REG_TESTPA1, 0x55)
		r.writeReg(REG_TESTPA2, 0x70)
		r.writeReg(REG_PALEVEL, settings.paLevel|RF_PALEVEL_PA1_ON)
		r.writeReg(REG_OCP, RF_OCP_ON)
	}
}

func (r *Radio) clearFIFO() {
	//r.writeReg(0x28, 0x10);
	r.writeReg(REG_IRQFLAGS2, RF_IRQFLAGS2_FIFOOVERRUN)
}

func (r *Radio) waitForPacketSent() {
	for r.readReg(REG_IRQFLAGS2)&RF_IRQFLAGS2_PACKETSENT == 0x00 {
	}
}
