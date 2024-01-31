package rfm69

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Packet) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Src":
			z.Src, err = dc.ReadByte()
			if err != nil {
				err = msgp.WrapError(err, "Src")
				return
			}
		case "Dst":
			z.Dst, err = dc.ReadByte()
			if err != nil {
				err = msgp.WrapError(err, "Dst")
				return
			}
		case "RSSI":
			z.RSSI, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "RSSI")
				return
			}
		case "Payload":
			z.Payload, err = dc.ReadBytes(z.Payload)
			if err != nil {
				err = msgp.WrapError(err, "Payload")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Packet) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "Src"
	err = en.Append(0x84, 0xa3, 0x53, 0x72, 0x63)
	if err != nil {
		return
	}
	err = en.WriteByte(z.Src)
	if err != nil {
		err = msgp.WrapError(err, "Src")
		return
	}
	// write "Dst"
	err = en.Append(0xa3, 0x44, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteByte(z.Dst)
	if err != nil {
		err = msgp.WrapError(err, "Dst")
		return
	}
	// write "RSSI"
	err = en.Append(0xa4, 0x52, 0x53, 0x53, 0x49)
	if err != nil {
		return
	}
	err = en.WriteInt(z.RSSI)
	if err != nil {
		err = msgp.WrapError(err, "RSSI")
		return
	}
	// write "Payload"
	err = en.Append(0xa7, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.Payload)
	if err != nil {
		err = msgp.WrapError(err, "Payload")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Packet) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "Src"
	o = append(o, 0x84, 0xa3, 0x53, 0x72, 0x63)
	o = msgp.AppendByte(o, z.Src)
	// string "Dst"
	o = append(o, 0xa3, 0x44, 0x73, 0x74)
	o = msgp.AppendByte(o, z.Dst)
	// string "RSSI"
	o = append(o, 0xa4, 0x52, 0x53, 0x53, 0x49)
	o = msgp.AppendInt(o, z.RSSI)
	// string "Payload"
	o = append(o, 0xa7, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64)
	o = msgp.AppendBytes(o, z.Payload)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Packet) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Src":
			z.Src, bts, err = msgp.ReadByteBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Src")
				return
			}
		case "Dst":
			z.Dst, bts, err = msgp.ReadByteBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Dst")
				return
			}
		case "RSSI":
			z.RSSI, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "RSSI")
				return
			}
		case "Payload":
			z.Payload, bts, err = msgp.ReadBytesBytes(bts, z.Payload)
			if err != nil {
				err = msgp.WrapError(err, "Payload")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Packet) Msgsize() (s int) {
	s = 1 + 4 + msgp.ByteSize + 4 + msgp.ByteSize + 5 + msgp.IntSize + 8 + msgp.BytesPrefixSize + len(z.Payload)
	return
}
