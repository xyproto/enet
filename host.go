package enet

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"os/signal"
	"time"
)

type EnetHost struct {
	fail         int // socket
	socket       *net.UDPConn
	addr         *net.UDPAddr
	incoming     chan *EnetHostIncomingCommand
	outgoing     chan *EnetHostOutgoingCommand
	tick         <-chan time.Time
	peers        map[string]*Peer
	timers       TimerQueue
	nextClientid uint32 // positive client id seed
	flags        int    // HostFlagsXxx
	rcvdBytes    int
	sentBytes    int
	rcvdBps      int
	sentBps      int
	updateEpoc   int64
	now          int64 // ms
	lastRecvTime int64
	lastSendTime int64

	notifyConnected    PeerEventHandler
	notifyDisconnected PeerEventHandler
	notifyData         DataEventHandler
}

func NewHost(addr string) (Host, error) {
	// if failed, host will bind to a random address
	ep, err := net.ResolveUDPAddr("udp", addr)

	host := &EnetHost{
		fail:     0,
		addr:     ep,
		incoming: make(chan *EnetHostIncomingCommand, 16),
		outgoing: make(chan *EnetHostOutgoingCommand, 16),
		tick:     time.Tick(time.Millisecond * DefaultTickMs),
		peers:    make(map[string]*Peer),
		timers:   newTimerQueue(),
	}
	if err == nil {
		host.socket, err = net.ListenUDP("udp", ep)
	}
	if err != nil {
		host.flags |= HostFlagsStopped
	}
	if host.addr == nil && host.socket != nil {
		host.addr = host.socket.LocalAddr().(*net.UDPAddr)
	}
	debugf("host bind %v\n", ep)
	return host, err
}

// process:
// - incoming packets
// - outgoing data
// - exit signal
// - timer tick
func (host *EnetHost) Run(sigs chan os.Signal) {
	host.flags |= HostFlagsRunning
	go host.runSocket()
	debugf("running...\n")
	for host.flags&HostFlagsStopped == 0 {
		select {
		case item := <-host.incoming:
			host.now = unixtimeNow()
			host.whenIncomingHostCommand(item)
		case item := <-host.outgoing:
			host.now = unixtimeNow()
			host.whenOutgoingHostCommand(item)
		case sig := <-sigs:
			host.now = unixtimeNow()
			signal.Stop(sigs)
			host.whenSignal(sig)
		case t := <-host.tick:
			host.now = unixtimeNow()
			host.whenTick(t)
		}
	}
	debugf("%v run exits\n", host.addr)
	host.flags &= ^HostFlagsRunning
}

func (host *EnetHost) Connect(ep string) {
	host.outgoing <- &EnetHostOutgoingCommand{ep, nil, ChannelIDAll, true}
}

func (host *EnetHost) Disconnect(ep string) {
	host.outgoing <- &EnetHostOutgoingCommand{ep, nil, ChannelIDNone, true}
}

func (host *EnetHost) Write(endp string, chanid uint8, dat []byte) {
	host.outgoing <- &EnetHostOutgoingCommand{endp, dat, chanid, true}
}

func (host *EnetHost) Stop() {
	host.whenSocketIncomingPacket(ProtocolHeader{}, PacketHeader{}, nil, nil)
}

// run in another routine
func (host *EnetHost) runSocket() {
	buf := make([]byte, UDPSize) // large enough

	sock := host.socket
	for {
		n, addr, err := sock.ReadFromUDP(buf)
		// syscall.EINVAL
		if err != nil { // any err will make host stop run
			break
		}
		dat := buf[:n]
		reader := bytes.NewReader(dat)
		var phdr ProtocolHeader
		binary.Read(reader, binary.BigEndian, &phdr)

		if phdr.Flags&ProtocolFlagsCrc != 0 {
			var crc32 CRC32Header
			binary.Read(reader, binary.BigEndian, &crc32)
		}

		var pkhdr PacketHeader
		for i := uint8(0); err == nil && i < phdr.PacketCount; i++ {
			err = binary.Read(reader, binary.BigEndian, &pkhdr)
			payload := make([]byte, int(pkhdr.Size)-binary.Size(pkhdr))
			_, err := reader.Read(payload)
			//debugf("socket recv %v\n", pkhdr)
			if err == nil {
				host.whenSocketIncomingPacket(phdr, pkhdr, payload, addr)
			}
		}

	}
	// socket may be not closed yet
	if host.flags&HostFlagsStopped == 0 {
		host.whenSocketIncomingPacket(ProtocolHeader{}, PacketHeader{}, nil, nil)
	}
}

func (host *EnetHost) whenSignal(sig os.Signal) {
	host.close()
}

func (host *EnetHost) close() {
	if host.flags&HostFlagsStopped != 0 {
		return
	}
	host.flags |= HostFlagsStopped

	assert(host.socket != nil)
	if host.flags&HostFlagsSockClosed == 0 {
		host.flags |= HostFlagsSockClosed
		host.socket.Close()
	}

	// disable tick func
	host.flags |= HostFlagsTickClosed
	debugf("%v closed\n", host.addr)
}

func (host *EnetHost) whenTick(t time.Time) {
	if host.flags&HostFlagsTickClosed != 0 {
		return
	}
	host.updateStatis()
	for cb := host.timers.pop(host.now); cb != nil; cb = host.timers.pop(host.now) {
		debugf("timer timeout\n")
		cb()
	}
}

// push data to socket
func (host *EnetHost) doSend(dat []byte, addr *net.UDPAddr) {
	assert(host.socket != nil)
	host.updateSNtStatis(len(dat))
	n, err := host.socket.WriteToUDP(dat, addr)
	assert(n == len(dat) || err != nil)
	if err != nil {
		host.close()
	}
	//	debugf("host do-send %v, %v, %v\n", n, err, addr)
}

// move rcvd socket datagrams to run routine
// payload or addr is nil means socket recv breaks
func (host *EnetHost) whenSocketIncomingPacket(phdr ProtocolHeader,
	pkhdr PacketHeader,
	payload []byte,
	addr *net.UDPAddr) (err error) {
	host.incoming <- &EnetHostIncomingCommand{
		phdr,
		pkhdr,
		payload,
		addr,
	}
	return
}

func (host *EnetHost) connectPeer(ep string) {
	debugf("connecting %v\n", ep)
	cid := host.nextClientid
	host.nextClientid++
	peer := host.peerFromEndpoint(ep, cid)
	if peer.clientid != cid { // connect a established peer?
		notifyPeerConnected(peer, PeerConnectResultDuplicated)
		return
	}
	peer.flags |= PeerFlagsSynSending
	hdr, syn := PacketSynDefault()
	ch := peer.channelFromID(ChannelIDNone)
	peer.outgoingPend(ch, hdr, PacketFragment{}, PacketSynEncode(syn))
}

func (host *EnetHost) disconnectPeer(ep string) {
	debugf("disconnecting %v\n", ep)
	peer := host.peerFromEndpoint(ep, PeerIDAny)
	if peer.flags&PeerFlagsEstablished == 0 {
		notifyPeerDisconnected(peer, PeerDisconnectResultInvalid)
		return
	}
	if peer.flags&(PeerFlagsFinRcvd|PeerFlagsFinSending) != 0 {
		return
	}
	hdr := PacketFinDefault()
	peer.flags |= PeerFlagsFinSending
	ch := peer.channelFromID(ChannelIDNone)
	peer.outgoingPend(ch, hdr, PacketFragment{}, []byte{})
}

func (host *EnetHost) resetPeer(ep string) {
	peer := host.peerFromEndpoint(ep, PeerIDAny)
	host.destroyPeer(peer)
}

func (host *EnetHost) whenOutgoingHostCommand(item *EnetHostOutgoingCommand) {
	if item.payload == nil {
		if item.chanid == ChannelIDAll { // connect request
			host.connectPeer(item.peer)
		}
		if item.chanid == ChannelIDNone { // disconnect
			host.disconnectPeer(item.peer)
		}
		return
	}
	peer := host.peerFromEndpoint(item.peer, PeerIDAny)
	if peer.flags&PeerFlagsEstablished == 0 ||
		peer.flags&(PeerFlagsFinSending|PeerFlagsSynackSending) != 0 {
		debugf("write denied for at status %X", peer.flags)
		return
	}
	ch := peer.channelFromID(item.chanid)
	l := uint32(len(item.payload))
	frags := (l + peer.mtu - 1) / peer.mtu
	firstsn := ch.NextSN
	if frags > 1 {
		for i := uint32(0); i < frags; i++ {
			dat := item.payload[i*peer.mtu : (i+1)*peer.mtu]
			pkhdr, frag := PacketFragmentDefault(item.chanid, uint32(len(dat)))
			frag.Count = frags
			frag.Index = i
			frag.Size = l
			frag.Offset = i * peer.mtu
			frag.SN = firstsn
			peer.outgoingPend(ch, pkhdr, frag, dat)
		}

	} else {
		pkhdr := PacketReliableDefault(item.chanid, l)
		peer.outgoingPend(ch, pkhdr, PacketFragment{}, item.payload)
	}
	//	ch.doSend(peer)
	return
}

func (host *EnetHost) whenIncomingHostCommand(item *EnetHostIncomingCommand) {
	if item == nil || item.payload == nil {
		host.close()
		return
	}
	host.updateRcvStatis(int(item.packetHeader.Size))

	if item.packetHeader.Type > PacketTypeCount {
		debugf("skipped packet: %v\n", item.packetHeader.Type)
		return
	}
	peer := host.peerFromAddr(item.endpoint, item.protocolHeader.ClientID)
	if peer.clientid != item.protocolHeader.ClientID {
		debugf("cid mismatch %v\n", peer.remoteAddr)
		return
	}
	ch := peer.channelFromID(item.packetHeader.ChannelID)

	// ack if needed
	if item.packetHeader.Flags&PacketHeaderFlagsNeedack != 0 {
		hdr, ack := PacketACKDefault(item.packetHeader.ChannelID)
		ack.SN = item.packetHeader.SN
		ack.SntTime = item.protocolHeader.SntTime
		debugf("ack packet %v, typ:%v\n", ack.SN, item.packetHeader.Type)
		peer.outgoingPend(ch, hdr, PacketFragment{}, PacketACKEncode(ack))
	}
	WhenPacketIncomingDisp[item.packetHeader.Type](peer, item.packetHeader, item.payload)
	//	ch.doSend(peer)
}

type whenPacketIncomingDisp func(peer *Peer, hdr PacketHeader, payload []byte)

var WhenPacketIncomingDisp = []whenPacketIncomingDisp{
	(*Peer).whenUnknown,
	(*Peer).whenIncomingACK,
	(*Peer).whenIncomingSyn,
	(*Peer).whenIncomingSynack,
	(*Peer).whenIncomingFin,
	(*Peer).whenIncomingPing,
	(*Peer).whenIncomingReliable,
	(*Peer).whenIncomingUnrelialbe,
	(*Peer).whenIncomingFragment,
	(*Peer).whenUnknown,
	(*Peer).whenUnknown,
	(*Peer).whenUnknown,
	(*Peer).whenIncomingEg,
	(*Peer).whenUnknown,
}

func (host *EnetHost) destroyPeer(peer *Peer) {
	id := peer.remoteAddr.String()
	delete(host.peers, id)
	debugf("release peer %v\n", id)
}

func (host *EnetHost) SetConnectionHandler(h PeerEventHandler) {
	host.notifyConnected = h
}

func (host *EnetHost) SetDisconnectionHandler(h PeerEventHandler) {
	host.notifyDisconnected = h
}

func (host *EnetHost) SetDataHandler(h DataEventHandler) {
	host.notifyData = h
}

func (host *EnetHost) updateRcvStatis(rcvd int) {
	host.rcvdBytes += rcvd
	host.lastRecvTime = host.now
}

func (host *EnetHost) updateSNtStatis(snt int) {
	host.sentBytes += snt
	host.lastSendTime = host.now
}

func (host *EnetHost) updateStatis() {
	itv := int(host.now - host.updateEpoc)
	host.rcvdBps = host.rcvdBytes * 1000 / itv
	host.sentBps = host.sentBytes * 1000 / itv
	host.rcvdBytes = 0
	host.sentBytes = 0
	for _, peer := range host.peers {
		peer.updateStatis(itv)
	}
}

const (
	HostFlagsNone = 1 << iota
	HostFlagsStopped
	HostFlagsRunning
	HostFlagsSockClosed
	HostFlagsTickClosed
)

type EnetHostIncomingCommand struct {
	protocolHeader ProtocolHeader
	packetHeader   PacketHeader // .size == len(payload)
	payload        []byte
	endpoint       *net.UDPAddr
}

type EnetHostOutgoingCommand struct {
	peer     string
	payload  []byte
	chanid   uint8
	reliable bool
}

func (host *EnetHost) peerFromEndpoint(ep string, clientid uint32) *Peer {
	addr, _ := net.ResolveUDPAddr("udp", ep)
	return host.peerFromAddr(addr, clientid)
}

func (host *EnetHost) peerFromAddr(ep *net.UDPAddr, clientid uint32) *Peer {
	assert(ep != nil)
	id := ep.String()
	peer, ok := host.peers[id]
	if !ok {
		peer = newPeer(ep, host)
		peer.clientid = clientid
		host.peers[id] = peer
	}
	return peer
}
