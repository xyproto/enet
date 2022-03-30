package enet

import (
	"bytes"
	"encoding/binary"
)

// Complete PacketHeader filling, no specific packetheader filling
func PacketACKDefault(chanid uint8) (hdr PacketHeader, ack PacketAck) {
	hdr.Type = PacketTypeACK
	hdr.Flags = 0
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr) + binary.Size(ack))
	return
}
func PacketACKEncode(ack PacketAck) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &ack)
	return writer.Bytes()
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketSynDefault() (hdr PacketHeader, syn PacketSyn) {
	syn.PeerID = 0
	syn.MTU = EnetdefaultMtu
	syn.WndSize = EnetdefaultWndsize
	syn.ChannelCount = EnetdefaultChannelCount
	syn.RcvBandwidth = 0
	syn.SndBandwidth = 0
	syn.ThrottleInterval = EnetdefaultThrottleInterval
	syn.ThrottleAcce = EnetdefaultThrottleAcce
	syn.ThrottleDece = EnetdefaultThrottleDece

	hdr.Type = PacketTypeSyn
	hdr.Flags = PacketHeaderFlagsNeedack
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr) + binary.Size(syn))
	return
}

func PacketSynEncode(syn PacketSyn) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &syn)
	return writer.Bytes()
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketSynackDefault() (hdr PacketHeader, sak PacketSynAck) {
	hdr, syn := PacketSynDefault()
	hdr.Type = PacketTypeSynack
	sak = PacketSynAck(syn)
	return
}
func PacketSynackEncode(sak PacketSynAck) []byte {
	writer := bytes.NewBuffer(nil)
	binary.Write(writer, binary.BigEndian, &sak)
	return writer.Bytes()
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketFinDefault() (hdr PacketHeader) {
	hdr.Type = PacketTypeFin
	hdr.Flags = PacketHeaderFlagsNeedack
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr))
	return
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketPingDefault(chanid uint8) (hdr PacketHeader) {
	hdr.Type = PacketTypePing
	hdr.Flags = PacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr))
	return
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketReliableDefault(chanid uint8, payloadlen uint32) (hdr PacketHeader) {
	hdr.Type = PacketTypeReliable
	hdr.Flags = PacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)) + payloadlen
	return
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketUnreliableDefault(chanid uint8, payloadlen, usn uint32) (hdr PacketHeader, pkt PacketUnreliable) {
	hdr.Type = PacketTypeUnreliable
	hdr.Flags = 0
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)+binary.Size(pkt)) + payloadlen
	pkt.SN = usn
	return
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketFragmentDefault(chanid uint8, fraglen uint32) (hdr PacketHeader, pkt PacketFragment) {
	hdr.Type = PacketTypeFragment
	hdr.Flags = PacketHeaderFlagsNeedack
	hdr.ChannelID = chanid
	hdr.Size = uint32(binary.Size(hdr)+binary.Size(pkt)) + fraglen
	return
}

// Complete PacketHeader filling, no specific packetheader filling
func PacketEgDefault() (hdr PacketHeader) {
	hdr.Type = PacketTypeFragment
	hdr.Flags = PacketHeaderFlagsNeedack // should be acked
	hdr.ChannelID = ChannelIDNone
	hdr.Size = uint32(binary.Size(hdr))
	return
}
