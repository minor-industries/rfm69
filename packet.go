package rfm69

//go:generate msgp

type Packet struct {
	Src     byte
	Dst     byte
	RSSI    int
	Payload []byte
}
