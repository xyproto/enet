package enet

import (
	"bytes"
	"encoding/binary"
	"net"
)

type Enetpeer struct {
	clientid         uint32 // local peer id
	mtu              uint32 // remote mtu
	sndBandwidth     uint32
	rcvBandwidth     uint32
	wndBytes         uint32
	chanCount        uint8
	throttle         uint32
	throttleInterval int64
	throttleAcce     uint32
	throttleDece     uint32
	rdata            uint
	ldata            uint // should use uint
	flags            int
	wndSize          uint32 // bytes, calculated by throttle and wndBytes
	intransBytes     int
	channel          [EnetdefaultChannelCount + 1]Channel
	remoteAddr       *net.UDPAddr
	rcvdBytes        int
	sentBytes        int
	rcvdBps          int
	sentBps          int
	rcvTimeo         int64
	rttTimeo         int64
	rtt              int64 // ms
	rttv             int64
	lastRtt          int64
	lastRttv         int64
	lowestRtt        int64
	highestRttv      int64
	rttEpoc          int64
	throttleEpoc     int64
	timeoutLimit     int64
	timeoutMin       int64
	timeoutMax       int64
	host             *EnetHost
}

func newEnetpeer(addr *net.UDPAddr, host *EnetHost) *Enetpeer {
	debugf("new peer %v\n", addr)
	cid := host.nextClientid
	host.nextClientid++
	return &Enetpeer{
		clientid:         cid,
		flags:            0,
		mtu:              EnetdefaultMtu,
		wndSize:          EnetdefaultWndsize,
		chanCount:        EnetdefaultChannelCount,
		throttle:         EnetdefaultThrottle,
		throttleInterval: EnetdefaultThrottleInterval,
		throttleAcce:     EnetdefaultThrottleAcce,
		throttleDece:     EnetdefaultThrottleDece,
		rcvTimeo:         5000,
		rtt:              EnetdefaultRtt,
		lastRtt:          EnetdefaultRtt,
		lowestRtt:        EnetdefaultRtt,
		rttEpoc:          0, // may expire as soon as fast
		throttleEpoc:     0, // may expire immediately
		timeoutLimit:     EnettimeoutLimit,
		timeoutMin:       EnettimeoutMin,
		timeoutMax:       EnettimeoutMax,
		remoteAddr:       addr,
		host:             host,
	}
}

func (peer *Enetpeer) doSend(hdr PacketHeader, frag PacketFragment, dat []byte) {
	writer := bytes.NewBuffer(nil)
	phdr := ProtocolHeader{0, 0, 1, uint32(peer.host.now), peer.clientid}
	binary.Write(writer, binary.BigEndian, phdr)
	binary.Write(writer, binary.BigEndian, &hdr)
	if hdr.Type == PacketTypeFragment {
		binary.Write(writer, binary.BigEndian, &frag)
	}
	binary.Write(writer, binary.BigEndian, dat)
	peer.host.doSend(writer.Bytes(), peer.remoteAddr)
	//	debugf("peer do-send %v\n", hdr.Type)
}

func (peer *Enetpeer) channelFromID(cid uint8) *Channel {
	if cid >= peer.chanCount {
		return &peer.channel[EnetdefaultChannelCount]
	}
	v := &peer.channel[cid]
	return v
}

func peerWindowIsFull(peer *Enetpeer) bool {
	return peer.intransBytes >= int(peer.wndSize)
}

func peerWindowIsEmpty(peer *Enetpeer) bool {
	return peer.intransBytes == 0
}

func (peer *Enetpeer) whenEnetincomingACK(header PacketHeader, payload []byte) {
	if peer.flags&EnetpeerFlagsStopped != 0 {
		return
	}
	reader := bytes.NewReader(payload)
	var ack PacketAck
	err := binary.Read(reader, binary.BigEndian, &ack)

	if err != nil {
		return
	}
	rtt := peer.host.now - int64(ack.SntTime)
	peer.updateRtt(rtt)
	peer.updateThrottle(rtt)
	debugf("peer in-ack %v\n", peer.remoteAddr)

	ch := peer.channelFromID(header.ChannelID)
	ch.outgoingACK(ack.SN)
	for i := ch.outgoingSlide(); i != nil; i = ch.outgoingSlide() {
		if i.retrans != nil {
			peer.host.timers.remove(i.retrans.index)
			i.retrans = nil
		}
		if i.header.Type == PacketTypeSyn {
			peer.flags |= EnetpeerFlagsSynSent
			if peer.flags&EnetpeerFlagsSynackRcvd != 0 {
				peer.flags |= EnetpeerFlagsEstablished
				notifyPeerConnected(peer, 0)
			}
		}
		if i.header.Type == PacketTypeSynack {
			peer.flags |= EnetpeerFlagsSynackSent
			if peer.flags&EnetpeerFlagsSynRcvd != 0 {
				peer.flags |= EnetpeerFlagsEstablished
				notifyPeerConnected(peer, 0)
			}
		}
		if i.header.Type == PacketTypeFin {
			peer.flags |= EnetpeerFlagsStopped
			notifyPeerDisconnected(peer, 0)
			peer.host.destroyPeer(peer)
		}
	}
}

func notifyData(peer *Enetpeer, dat []byte) {
	debugf("on-dat %v\n", len(dat))
}

func notifyPeerConnected(peer *Enetpeer, ret int) {
	debugf("peer connected %v, ret: %v\n", peer.remoteAddr, ret)
}

func notifyPeerDisconnected(peer *Enetpeer, ret int) {
	debugf("peer disconnected %v, ret: %v\n", peer.remoteAddr, ret)
}

func (peer *Enetpeer) reset() {}

func (peer *Enetpeer) handshake(syn PacketSyn) {}

func (peer *Enetpeer) whenEnetincomingSyn(header PacketHeader, payload []byte) {
	debugf("peer in-syn %v\n", peer.remoteAddr)
	reader := bytes.NewReader(payload)
	var syn PacketSyn
	err := binary.Read(reader, binary.BigEndian, &syn)

	if err != nil || peer.flags&EnetpeerFlagsSynackSending != 0 {
		return
	}
	if peer.flags&(EnetpeerFlagsSynSent|EnetpeerFlagsSynRcvd) != 0 {
		peer.reset()
	}
	peer.handshake(syn)
	// send synack
	peer.flags |= EnetpeerFlagsSynackSending
	peer.flags |= EnetpeerFlagsSynRcvd
	ch := peer.channelFromID(ChannelIDNone)
	phdr, synack := PacketSynackDefault()

	// todo add retrans timer
	peer.outgoingPend(ch, phdr, PacketFragment{}, PacketSynackEncode(synack))
}

func (peer *Enetpeer) whenEnetincomingSynack(header PacketHeader, payload []byte) {
	debugf("peer in-synack %v\n", peer.remoteAddr)
	reader := bytes.NewReader(payload)
	var syn PacketSyn
	err := binary.Read(reader, binary.BigEndian, &syn)

	if err != nil || peer.flags&EnetpeerFlagsSynSending == 0 {
		debugf("peer reset %X", peer.flags)
		peer.reset()
		return
	}
	peer.handshake(syn)
	peer.flags |= EnetpeerFlagsSynackRcvd
	if peer.flags&EnetpeerFlagsSynSent != 0 {
		peer.flags |= EnetpeerFlagsEstablished
		notifyPeerConnected(peer, 0)
	}
}

func (peer *Enetpeer) whenEnetincomingFin(header PacketHeader, payload []byte) {
	debugf("peer in-fin %v\n", peer.remoteAddr)
	if peer.flags&EnetpeerFlagsFinSending != 0 {
		// needn't do anything, just wait for self fin's ack
		return
	}
	peer.flags |= EnetpeerFlagsFinRcvd | EnetpeerFlagsLastack // enter time-wait state
	notifyPeerDisconnected(peer, 0)

	peer.host.timers.push(peer.host.now+peer.rttTimeo*2, func() { peer.host.destroyPeer(peer) })
}

func (peer *Enetpeer) whenEnetincomingPing(header PacketHeader, payload []byte) {}

func (peer *Enetpeer) whenEnetincomingReliable(header PacketHeader, payload []byte) {
	debugf("peer in-reliable %v\n", peer.remoteAddr)
	if peer.flags&EnetpeerFlagsEstablished == 0 {
		return
	}
	ch := peer.channelFromID(header.ChannelID)
	if ch == nil {
		return
	}
	ch.incomingTrans(&ChannelItem{header, PacketFragment{}, payload, 0, 0, nil})
	ch.incomingACK(header.SN)
	for i := ch.incomingSlide(); i != nil; i = ch.incomingSlide() {
		notifyData(peer, i.payload)
	}
}

func (peer *Enetpeer) whenEnetincomingFragment(header PacketHeader, payload []byte) {
	debugf("peer in-frag %v\n", peer.remoteAddr)
	reader := bytes.NewReader(payload)
	var frag PacketFragment
	binary.Read(reader, binary.BigEndian, &frag)

	dat := make([]byte, int(header.Size)-binary.Size(frag))
	reader.Read(dat)
	ch := peer.channelFromID(header.ChannelID)
	if ch == nil {
		return
	}
	ch.incomingTrans(&ChannelItem{header, frag, dat, 0, 0, nil})
	ch.incomingACK(header.SN)
	for i := ch.incomingSlide(); i != nil; i = ch.incomingSlide() {
		notifyData(peer, i.payload)
	}
}

func (peer *Enetpeer) whenEnetincomingUnrelialbe(header PacketHeader, payload []byte) {
	debugf("peer in-unreliable %v\n", peer.remoteAddr)
	reader := bytes.NewReader(payload)
	var ur PacketUnreliable
	binary.Read(reader, binary.BigEndian, &ur)

	dat := make([]byte, int(header.Size)-binary.Size(ur))
	reader.Read(dat)
	notifyData(peer, dat)
}
func (peer *Enetpeer) whenUnknown(header PacketHeader, payload []byte) {
	debugf("peer skipped packet : %v\n", header.Type)
}
func (peer *Enetpeer) whenEnetincomingEg(header PacketHeader, payload []byte) {
	debugf("peer in-eg %v\n", peer.remoteAddr)

}

const (
	EnetpeerFlagsNone        = 1 << iota // never used
	EnetpeerFlagsSockClosed              // sock is closed
	EnetpeerFlagsStopped                 // closed, rcvd fin, and sent fin+ack and then rcvd fin+ack's ack
	EnetpeerFlagsLastack                 // send fin's ack, and waiting retransed fin in rtttimeout
	EnetpeerFlagsSynSending              // connecting            sync-sent
	EnetpeerFlagsSynSent                 // syn acked
	EnetpeerFlagsSynRcvd                 // acking-connect        sync-rcvd
	EnetpeerFlagsListening               // negative peer
	EnetpeerFlagsEstablished             // established
	EnetpeerFlagsFinSending              // sent fin, waiting the ack
	EnetpeerFlagsFinSent                 // rcvd fin's ack
	EnetpeerFlagsFinRcvd                 //
	EnetpeerFlagsNothing
	EnetpeerFlagsSynackSending
	EnetpeerFlagsSynackRcvd
	EnetpeerFlagsSynackSent
)

func (peer *Enetpeer) updateRtt(rtt int64) {
	v := rtt - peer.rtt
	peer.rtt += v / 8
	peer.rttv = peer.rttv - peer.rttv/4 + absi64(v/4)

	peer.lowestRtt = mini64(peer.lowestRtt, peer.rtt)
	peer.highestRttv = maxi64(peer.highestRttv, peer.rttv)

	if peer.host.now > peer.throttleInterval+peer.throttleEpoc {
		peer.throttleEpoc = peer.host.now
		peer.lastRtt = peer.lowestRtt
		peer.lastRttv = peer.highestRttv
		peer.lowestRtt = peer.rtt
		peer.highestRttv = peer.rttv
	}
	peer.rttTimeo = rtt + peer.rttv<<2
	peer.rcvTimeo = peer.rttTimeo << 4
}

func (peer *Enetpeer) updateThrottle(rtt int64) {
	// unstable network
	if peer.lastRtt <= peer.lastRttv {
		peer.throttle = EnetthrottleScale
		return
	}
	if rtt < peer.lastRtt {
		peer.throttle = minui32(peer.throttle+peer.throttleAcce, EnetthrottleScale)
		return
	}
	if rtt > peer.lastRtt+peer.lastRttv<<1 {
		peer.throttle = maxui32(peer.throttle-peer.throttleDece, 0)
	}
}

func (peer *Enetpeer) updateWindowSize() {
	peer.wndSize = peer.wndBytes * peer.throttle / EnetthrottleScale
}

func (peer *Enetpeer) updateStatis(itv int) {
	peer.rcvdBps = peer.rcvdBytes * 1000 / itv
	peer.rcvdBytes = 0
	peer.sentBps = peer.sentBytes * 1000 / itv
	peer.sentBytes = 0
}
func (peer *Enetpeer) Addr() net.Addr {
	return peer.remoteAddr
}

// do with 2 times retry
func (peer *Enetpeer) outgoingPend(ch *Channel,
	hdr PacketHeader,
	frag PacketFragment,
	dat []byte) {
	item := &ChannelItem{hdr, frag, dat, 0, 0, nil}
	reset := func() {

	}
	reretrans := func() {
		item.retrans = peer.host.timers.push(peer.host.now+peer.rcvTimeo, reset)
		item.retries++
		peer.doSend(item.header, item.fragment, item.payload)
	}
	retrans := func() {
		item.retrans = peer.host.timers.push(peer.host.now+peer.rcvTimeo, reretrans)
		item.retries++
		peer.doSend(item.header, item.fragment, item.payload)
	}

	if hdr.Type != PacketTypeACK {
		item.retrans = peer.host.timers.push(peer.host.now+peer.rcvTimeo, retrans)
	}
	ch.outgoingPend(item)
	ch.doSend(peer)
}
