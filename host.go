package enet

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"os/signal"
	"time"
)

type Enethost struct {
	fail         int // socket
	socket       *net.UDPConn
	addr         *net.UDPAddr
	incoming     chan *EnethostIncomingCommand
	outgoing     chan *EnethostOutgoingCommand
	tick         <-chan time.Time
	peers        map[string]*Enetpeer
	timers       EnetTimerQueue
	nextClientid uint32 // positive client id seed
	flags        int    // EnethostFlagsXxx
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

	host := &Enethost{
		fail:     0,
		addr:     ep,
		incoming: make(chan *EnethostIncomingCommand, 16),
		outgoing: make(chan *EnethostOutgoingCommand, 16),
		tick:     time.Tick(time.Millisecond * EnetdefaultTickMs),
		peers:    make(map[string]*Enetpeer),
		timers:   newEnetTimerQueue(),
	}
	if err == nil {
		host.socket, err = net.ListenUDP("udp", ep)
	}
	if err != nil {
		host.flags |= EnethostFlagsStopped
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
func (host *Enethost) Run(sigs chan os.Signal) {
	host.flags |= EnethostFlagsRunning
	go host.runSocket()
	debugf("running...\n")
	for host.flags&EnethostFlagsStopped == 0 {
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
	host.flags &= ^EnethostFlagsRunning
}

func (host *Enethost) Connect(ep string) {
	host.outgoing <- &EnethostOutgoingCommand{ep, nil, ChannelIDAll, true}
}
func (host *Enethost) Disconnect(ep string) {
	host.outgoing <- &EnethostOutgoingCommand{ep, nil, ChannelIDNone, true}
}

func (host *Enethost) Write(endp string, chanid uint8, dat []byte) {
	host.outgoing <- &EnethostOutgoingCommand{endp, dat, chanid, true}
}
func (host *Enethost) Stop() {
	host.whenSocketIncomingPacket(EnetProtocolHeader{}, PacketHeader{}, nil, nil)
}

// run in another routine
func (host *Enethost) runSocket() {
	buf := make([]byte, EnetudpSize) // large enough

	sock := host.socket
	for {
		n, addr, err := sock.ReadFromUDP(buf)
		// syscall.EINVAL
		if err != nil { // any err will make host stop run
			break
		}
		dat := buf[:n]
		reader := bytes.NewReader(dat)
		var phdr EnetProtocolHeader
		binary.Read(reader, binary.BigEndian, &phdr)

		if phdr.Flags&EnetprotocolFlagsCrc != 0 {
			var crc32 EnetCrc32Header
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
	if host.flags&EnethostFlagsStopped == 0 {
		host.whenSocketIncomingPacket(EnetProtocolHeader{}, PacketHeader{}, nil, nil)
	}
}

func (host *Enethost) whenSignal(sig os.Signal) {
	host.close()
}
func (host *Enethost) close() {
	if host.flags&EnethostFlagsStopped != 0 {
		return
	}
	host.flags |= EnethostFlagsStopped

	assert(host.socket != nil)
	if host.flags&EnethostFlagsSockClosed == 0 {
		host.flags |= EnethostFlagsSockClosed
		host.socket.Close()
	}

	// disable tick func
	host.flags |= EnethostFlagsTickClosed
	debugf("%v closed\n", host.addr)
}
func (host *Enethost) whenTick(t time.Time) {
	if host.flags&EnethostFlagsTickClosed != 0 {
		return
	}
	host.updateStatis()
	for cb := host.timers.pop(host.now); cb != nil; cb = host.timers.pop(host.now) {
		debugf("timer timeout\n")
		cb()
	}
}

// push data to socket
func (host *Enethost) doSend(dat []byte, addr *net.UDPAddr) {
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
func (host *Enethost) whenSocketIncomingPacket(phdr EnetProtocolHeader,
	pkhdr PacketHeader,
	payload []byte,
	addr *net.UDPAddr) (err error) {
	host.incoming <- &EnethostIncomingCommand{
		phdr,
		pkhdr,
		payload,
		addr,
	}
	return
}
func (host *Enethost) connectPeer(ep string) {
	debugf("connecting %v\n", ep)
	cid := host.nextClientid
	host.nextClientid++
	peer := host.peerFromEndpoint(ep, cid)
	if peer.clientid != cid { // connect a established peer?
		notifyPeerConnected(peer, EnetpeerConnectResultDuplicated)
		return
	}
	peer.flags |= EnetpeerFlagsSynSending
	hdr, syn := EnetpacketSynDefault()
	ch := peer.channelFromID(ChannelIDNone)
	peer.outgoingPend(ch, hdr, PacketFragment{}, EnetpacketSynEncode(syn))
}
func (host *Enethost) disconnectPeer(ep string) {
	debugf("disconnecting %v\n", ep)
	peer := host.peerFromEndpoint(ep, EnetpeerIDAny)
	if peer.flags&EnetpeerFlagsEstablished == 0 {
		notifyPeerDisconnected(peer, EnetpeerDisconnectResultInvalid)
		return
	}
	if peer.flags&(EnetpeerFlagsFinRcvd|EnetpeerFlagsFinSending) != 0 {
		return
	}
	hdr := EnetpacketFinDefault()
	peer.flags |= EnetpeerFlagsFinSending
	ch := peer.channelFromID(ChannelIDNone)
	peer.outgoingPend(ch, hdr, PacketFragment{}, []byte{})
}
func (host *Enethost) resetPeer(ep string) {
	peer := host.peerFromEndpoint(ep, EnetpeerIDAny)
	host.destroyPeer(peer)
}
func (host *Enethost) whenOutgoingHostCommand(item *EnethostOutgoingCommand) {
	if item.payload == nil {
		if item.chanid == ChannelIDAll { // connect request
			host.connectPeer(item.peer)
		}
		if item.chanid == ChannelIDNone { // disconnect
			host.disconnectPeer(item.peer)
		}
		return
	}
	peer := host.peerFromEndpoint(item.peer, EnetpeerIDAny)
	if peer.flags&EnetpeerFlagsEstablished == 0 ||
		peer.flags&(EnetpeerFlagsFinSending|EnetpeerFlagsSynackSending) != 0 {
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
			pkhdr, frag := EnetpacketFragmentDefault(item.chanid, uint32(len(dat)))
			frag.Count = frags
			frag.Index = i
			frag.Size = l
			frag.Offset = i * peer.mtu
			frag.SN = firstsn
			peer.outgoingPend(ch, pkhdr, frag, dat)
		}

	} else {
		pkhdr := EnetpacketReliableDefault(item.chanid, l)
		peer.outgoingPend(ch, pkhdr, PacketFragment{}, item.payload)
	}
	//	ch.doSend(peer)
	return
}

func (host *Enethost) whenIncomingHostCommand(item *EnethostIncomingCommand) {
	if item == nil || item.payload == nil {
		host.close()
		return
	}
	host.updateRcvStatis(int(item.packetHeader.Size))

	if item.packetHeader.Type > EnetpacketTypeCount {
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
	if item.packetHeader.Flags&EnetpacketHeaderFlagsNeedack != 0 {
		hdr, ack := EnetpacketACKDefault(item.packetHeader.ChannelID)
		ack.SN = item.packetHeader.SN
		ack.SntTime = item.protocolHeader.SntTime
		debugf("ack packet %v, typ:%v\n", ack.SN, item.packetHeader.Type)
		peer.outgoingPend(ch, hdr, PacketFragment{}, EnetpacketACKEncode(ack))
	}
	WhenEnetpacketIncomingDisp[item.packetHeader.Type](peer, item.packetHeader, item.payload)
	//	ch.doSend(peer)
}

type whenEnetpacketIncomingDisp func(peer *Enetpeer, hdr PacketHeader, payload []byte)

var WhenEnetpacketIncomingDisp = []whenEnetpacketIncomingDisp{
	(*Enetpeer).whenUnknown,
	(*Enetpeer).whenEnetincomingACK,
	(*Enetpeer).whenEnetincomingSyn,
	(*Enetpeer).whenEnetincomingSynack,
	(*Enetpeer).whenEnetincomingFin,
	(*Enetpeer).whenEnetincomingPing,
	(*Enetpeer).whenEnetincomingReliable,
	(*Enetpeer).whenEnetincomingUnrelialbe,
	(*Enetpeer).whenEnetincomingFragment,
	(*Enetpeer).whenUnknown,
	(*Enetpeer).whenUnknown,
	(*Enetpeer).whenUnknown,
	(*Enetpeer).whenEnetincomingEg,
	(*Enetpeer).whenUnknown,
}

func (host *Enethost) destroyPeer(peer *Enetpeer) {
	id := peer.remoteAddr.String()
	delete(host.peers, id)
	debugf("release peer %v\n", id)
}
func (host *Enethost) SetConnectionHandler(h PeerEventHandler) {
	host.notifyConnected = h
}
func (host *Enethost) SetDisconnectionHandler(h PeerEventHandler) {
	host.notifyDisconnected = h
}

func (host *Enethost) SetDataHandler(h DataEventHandler) {
	host.notifyData = h
}
func (host *Enethost) updateRcvStatis(rcvd int) {
	host.rcvdBytes += rcvd
	host.lastRecvTime = host.now
}
func (host *Enethost) updateSNtStatis(snt int) {
	host.sentBytes += snt
	host.lastSendTime = host.now
}

func (host *Enethost) updateStatis() {
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
	EnethostFlagsNone = 1 << iota
	EnethostFlagsStopped
	EnethostFlagsRunning
	EnethostFlagsSockClosed
	EnethostFlagsTickClosed
)

type EnethostIncomingCommand struct {
	protocolHeader EnetProtocolHeader
	packetHeader   PacketHeader // .size == len(payload)
	payload        []byte
	endpoint       *net.UDPAddr
}
type EnethostOutgoingCommand struct {
	peer     string
	payload  []byte
	chanid   uint8
	reliable bool
}

func (host *Enethost) peerFromEndpoint(ep string, clientid uint32) *Enetpeer {
	addr, _ := net.ResolveUDPAddr("udp", ep)
	return host.peerFromAddr(addr, clientid)
}
func (host *Enethost) peerFromAddr(ep *net.UDPAddr, clientid uint32) *Enetpeer {
	assert(ep != nil)
	id := ep.String()
	peer, ok := host.peers[id]
	if !ok {
		peer = newEnetpeer(ep, host)
		peer.clientid = clientid
		host.peers[id] = peer
	}

	return peer
}
