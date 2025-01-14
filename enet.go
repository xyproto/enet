package enet

/*
udpHeader | protocolHeader | crc32Header? | ( packetHeader | payload )+
*/
type ProtocolHeader struct {
	PeerID      uint16 // target peerid, not used
	Flags       uint8  // 0xcc : use crc32 header, default 0
	PacketCount uint8  // Packets in this datagram
	SntTime     uint32 // milli-second, sent-time
	ClientID    uint32 // client-id? , server would fill client's id, not his own
}
type CRC32Header struct {
	CRC32 uint32
}
type PacketHeader struct {
	Type      uint8  // PacketTypeXxx
	Flags     uint8  // Needack, Forcefin, PacketHeaderFlagsXxx
	ChannelID uint8  // [0,n), or 0xff; oxff : control channel
	RSV       uint8  // not used
	Size      uint32 // including packetHeader and payload, bytes
	SN        uint32 // used for any packet type which should be acked, not used for unreliable, ack
}

//cmdTypeACK = 1
// flags must be zero
type PacketAck struct {
	SN      uint32 // rcvd-sn // not the next sn
	SntTime uint32 // rcvd sent time
}

//cmdTypeSyn = 2
// flags = PacketNeedack
type PacketSyn struct { // ack by conack
	PeerID           uint16 // zero, whose id write the packet
	MTU              uint16 // default = 1200
	WndSize          uint32 // local recv window size, 0x8000
	ChannelCount     uint32 // channels count, default = 2
	RcvBandwidth     uint32 // local receiving bandwith bps, 0 means no limit
	SndBandwidth     uint32 // local sending bandwidth , 0 means no limit
	ThrottleInterval uint32 // = 0x1388 = 5000ms
	ThrottleAcce     uint32 // = 2
	ThrottleDece     uint32 // = 2
}

//cmdTypeSynack = 3
// flags = PacketNeedack
type PacketSynAck PacketSyn

//cmdTypeFin = 4
// flags = PacketFlagsForcefin if disconnect unconnected peer
type PacketFin struct{}

// cmdType = 5
type PacketPing struct{}

//cmdTypeReliable = 6
// flags= PacketHeaderFlagsNeedack
type PacketReliable struct{}

//cmdTypeUnreliable = 7
// flags = PacketHeaderFlagsNone
type PacketUnreliable struct {
	SN uint32 // unreliable sequence number, filled with channel.nextUSN
}

//cmdTypeFragment = 8
// [offset, length) of the packet sn
// packet was splitted into fragmentCount parts
type PacketFragment struct {
	SN     uint32 // start sequence number
	Count  uint32 // fragment counts
	Index  uint32 // index of current fragment
	Size   uint32 // total length of all fragments
	Offset uint32
}
