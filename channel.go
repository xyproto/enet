package enet

const channelPacketCount = 256

type EnetChannelItem struct {
	header   EnetPacketHeader
	fragment EnetPacketFragment // used if header.cmd == EnetpacketFragment
	payload  []byte             // not include packet-header
	retries  int                // sent times for outgoing packet
	acked    int                // acked times
	retrans  *EnetTimerItem     // retrans timer
}

// outgoing: ->end ..untransfered.. next ..transfered.. begin ->
// incoming: <-begin ..acked.. next ..unacked.. end<-
type EnetChannel struct {
	NextSN        uint32 // next reliable packet number for sent
	NextUSN       uint32 // next unsequenced packet number for sent
	outgoing      [channelPacketCount]*EnetChannelItem
	incoming      [channelPacketCount]*EnetChannelItem
	outgoingBegin uint32 // the first one is not acked yet
	incomingBegin uint32 // the first one has be received
	outgoingEnd   uint32 // the last one is not acked yet
	incomingEnd   uint32 // the last one has been received
	outgoingUsed  uint32 // in trans packets not acked
	incomingUsed  uint32 // rcvd packet count in incoming window
	outgoingNext  uint32 // the next one is being to send first time
	intransBytes  uint32
}

func (ch *EnetChannel) outgoingPend(item *EnetChannelItem) {
	item.header.SN = ch.NextSN
	ch.NextSN++
	debugf("channel outgoing %v, typ: %v\n", item.header.SN, item.header.Type)
	idx := item.header.SN % channelPacketCount
	v := ch.outgoing[idx]
	assert(v == nil && item.header.SN == ch.outgoingEnd)
	ch.outgoing[idx] = item
	if ch.outgoingEnd <= item.header.SN {
		ch.outgoingEnd = item.header.SN + 1
	}
	ch.outgoingUsed++
}

// what if outgoingWrap
func (ch *EnetChannel) outgoingACK(sn uint32) {
	debugf("outgoing ack %v\n", sn)
	if sn < ch.outgoingBegin || sn >= ch.outgoingEnd { // already acked or error
		debugf("channel-ack abandoned %v\n", sn)
		return
	}
	idx := sn % channelPacketCount
	v := ch.outgoing[idx]
	assert(v != nil && v.header.SN == sn)
	ch.intransBytes -= v.header.Size
	v.acked++
}

func (ch *EnetChannel) outgoingSlide() (item *EnetChannelItem) {
	assert(ch.outgoingBegin <= ch.outgoingEnd)
	if ch.outgoingBegin >= ch.outgoingEnd {
		return
	}
	idx := ch.outgoingBegin % channelPacketCount
	v := ch.outgoing[idx]
	assert(v != nil)
	if v.retries == 0 {
		return
	}
	if v.header.Type != EnetpacketTypeACK && v.acked == 0 {
		return
	}
	debugf("outgoing slide %v, sn:%v, rty:%v, ack:%v\n", v.header.Type, v.header.SN, v.retries, v.acked)
	item = v
	ch.outgoingBegin++
	return
}

// the first time send out packet
func (ch *EnetChannel) outgoingDoTrans() (item *EnetChannelItem) {
	assert(ch.outgoingNext <= ch.outgoingEnd)
	if ch.outgoingNext >= ch.outgoingEnd {
		return
	}
	idx := ch.outgoingNext % channelPacketCount
	item = ch.outgoing[idx]
	assert(item != nil)
	assert((item.acked == 0 && item.header.Type != EnetpacketTypeACK) || item.header.Type == EnetpacketTypeACK)
	item.retries++
	ch.outgoingNext++
	ch.intransBytes += item.header.Size
	return
}

// may be retransed packet
func (ch *EnetChannel) incomingTrans(item *EnetChannelItem) {
	if item.header.SN < ch.incomingBegin {
		return
	}
	idx := item.header.SN % channelPacketCount
	v := ch.incoming[idx]
	// duplicated packet
	if v != nil {
		v.retries++
		return
	}
	assert(v == nil || v.header.SN == item.header.SN)

	ch.incoming[idx] = item
	ch.incomingUsed++
	if ch.incomingEnd <= item.header.SN {
		ch.incomingEnd = item.header.SN + 1
	}
}

// when do ack incoming packets
func (ch *EnetChannel) incomingACK(sn uint32) {
	if sn < ch.incomingBegin || sn >= ch.incomingEnd { // reack packet not in wnd
		return
	}
	idx := sn % channelPacketCount
	v := ch.incoming[idx]
	assert(v != nil && v.header.SN == sn)
	v.acked++
}

// called after incoming-ack
func (ch *EnetChannel) incomingSlide() (item *EnetChannelItem) { // return value may be ignored
	if ch.incomingBegin >= ch.incomingEnd {
		return
	}
	idx := ch.incomingBegin % channelPacketCount
	v := ch.incoming[idx]
	if v == nil || v.acked <= 0 { // not received yet
		return
	}
	assert(v.header.SN == ch.incomingBegin)

	// merge fragments
	if v.header.Type == EnetpacketTypeFragment {
		all := true
		for i := uint32(1); i < v.fragment.Count; i++ {
			n := ch.incoming[idx+i]
			if n == nil || n.header.SN != v.header.SN+i || n.fragment.SN != v.header.SN {
				all = false
				break
			}
		}
		if !all {
			return
		}

		item = v
		ch.incomingBegin += v.fragment.Count
		ch.incomingUsed -= v.fragment.Count
		for i := uint32(1); i < v.fragment.Count; i++ {
			item.payload = append(item.payload, ch.incoming[idx+1].payload...)
			ch.incoming[idx+i] = nil
		}
		ch.incoming[idx] = nil

		return
	}
	item = v
	ch.incomingBegin++
	ch.incomingUsed--
	ch.incoming[idx] = nil
	return
}

func (ch *EnetChannel) doSend(peer *Enetpeer) {
	if ch.intransBytes > peer.wndSize { // window is overflow
		return
	}
	for item := ch.outgoingDoTrans(); item != nil; item = ch.outgoingDoTrans() {
		peer.doSend(item.header, item.fragment, item.payload)
		item.retries++
	}
}
