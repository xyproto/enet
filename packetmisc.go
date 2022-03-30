package enet

import (
	"bytes"
	"encoding/binary"
)

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketACKDefault(chanid uint8) (hdr PacketHeader, ack PacketAck) {
	hdr.Type = EnetpacketTypeACK
	hdr.Flags = 0
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr) + binary.Size(ack))
	return
}
func EnetpacketACKEncode(ack PacketAck) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &ack)
	return writer.Bytes()
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketSynDefault() (hdr PacketHeader, syn PacketSyn) {
	syn.PeerID = 0
	syn.MTU = EnetdefaultMtu
	syn.WndSize = EnetdefaultWndsize
	syn.ChannelCount = EnetdefaultChannelCount
	syn.RcvBandwidth = 0
	syn.SndBandwidth = 0
	syn.ThrottleInterval = EnetdefaultThrottleInterval
	syn.ThrottleAcce = EnetdefaultThrottleAcce
	syn.ThrottleDece = EnetdefaultThrottleDece

	hdr.Type = EnetpacketTypeSyn
	hdr.Flags = EnetpacketHeaderFlagsNeedack
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr) + binary.Size(syn))
	return
}
func EnetpacketSynEncode(syn PacketSyn) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &syn)
	return writer.Bytes()
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketSynackDefault() (hdr PacketHeader, sak PacketSynAck) {
	hdr, syn := EnetpacketSynDefault()
	hdr.Type = EnetpacketTypeSynack
	sak = PacketSynAck(syn)
	return
}
func EnetpacketSynackEncode(sak PacketSynAck) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &sak)
	return writer.Bytes()
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketFinDefault() (hdr PacketHeader) {
	hdr.Type = EnetpacketTypeFin
	hdr.Flags = EnetpacketHeaderFlagsNeedack
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr))
	return
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketPingDefault(chanid uint8) (hdr PacketHeader) {
	hdr.Type = EnetpacketTypePing
	hdr.Flags = EnetpacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr))
	return
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketReliableDefault(chanid uint8, payloadlen uint32) (hdr PacketHeader) {
	hdr.Type = EnetpacketTypeReliable
	hdr.Flags = EnetpacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)) + payloadlen
	return
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketUnreliableDefault(chanid uint8, payloadlen, usn uint32) (hdr PacketHeader, pkt PacketUnreliable) {
	hdr.Type = EnetpacketTypeUnreliable
	hdr.Flags = 0
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)+binary.Size(pkt)) + payloadlen
	pkt.SN = usn
	return
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketFragmentDefault(chanid uint8, fraglen uint32) (hdr PacketHeader, pkt PacketFragment) {
	hdr.Type = EnetpacketTypeFragment
	hdr.Flags = EnetpacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)+binary.Size(pkt)) + fraglen
	return
}

// 完成 EnetpacketHeader的填充，没有具体的packetheader填充
func EnetpacketEgDefault() (hdr PacketHeader) {
	hdr.Type = EnetpacketTypeFragment
	hdr.Flags = EnetpacketHeaderFlagsNeedack // should be acked
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr))
	return
}
