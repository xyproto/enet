package enet

const (
	PacketTypeUnspec     uint8 = iota
	PacketTypeACK              = 1
	PacketTypeSyn              = 2
	PacketTypeSynack           = 3
	PacketTypeFin              = 4
	PacketTypePing             = 5
	PacketTypeReliable         = 6
	PacketTypeUnreliable       = 7
	PacketTypeFragment         = 8
	PacketTypeEg               = 12
	PacketTypeCount            = 12
)

const (
	EnetprotocolFlagsNone uint8 = iota
	EnetprotocolFlagsCrc        = 0xcc // use Enetcrc32Header
)

const (
	PacketHeaderFlagsNone     uint8 = iota
	PacketHeaderFlagsNeedack        // for syn, syncak, fin, reliable, ping, fragment
	PacketHeaderFlagsForcefin       // i don't know how to use this flag
)

const (
	DefaultMtu              = 1400
	DefaultWndsize          = 0x8000 // bytes
	DefaultChannelCount     = 2
	DefaultThrottleInterval = 5000 // ms
	DefaultThrottleAcce     = 2
	DefaultThrottleDece     = 2
	DefaultTickMs           = 20  // ms
	DefaultRtt              = 500 //ms
	EnetudpSize             = 65536
	DefaultThrottle         = 32
	ThrottleScale           = 32
	EnettimeoutLimit        = 32   // 30 seconds
	EnettimeoutMin          = 5000 // 5 second
	EnettimeoutMax          = 30000
	EnetpingInterval        = 1000 // 1 second
	EnetbpsInterval         = 1000 // 1 second
)

const (
	ChannelIDNone uint8 = 0xff
	ChannelIDAll        = 0xfe
)

const (
	PeerConnectResultDuplicated = 1
	PeerDisconnectResultInvalid
)

const PeerIDAny uint32 = 0xffffffff

/* uncompatiable with enet origin protocol
EnetcmdUnsequenced    // +unseq flag
EnetcmdBandwidthlimit // ack flag
EnetcmdThrottle       // ack flag
EnetcmdUnreliableFragment
*/
