package enet

const (
	EnetpacketTypeUnspec     uint8 = iota
	EnetpacketTypeACK              = 1
	EnetpacketTypeSyn              = 2
	EnetpacketTypeSynack           = 3
	EnetpacketTypeFin              = 4
	EnetpacketTypePing             = 5
	EnetpacketTypeReliable         = 6
	EnetpacketTypeUnreliable       = 7
	EnetpacketTypeFragment         = 8
	EnetpacketTypeEg               = 12
	EnetpacketTypeCount            = 12
)

const (
	EnetprotocolFlagsNone uint8 = iota
	EnetprotocolFlagsCrc        = 0xcc // use Enetcrc32Header
)

const (
	EnetpacketHeaderFlagsNone     uint8 = iota
	EnetpacketHeaderFlagsNeedack        // for syn, syncak, fin, reliable, ping, fragment
	EnetpacketHeaderFlagsForcefin       // i don't know how to use this flag
)

const (
	EnetdefaultMtu              = 1400
	EnetdefaultWndsize          = 0x8000 // bytes
	EnetdefaultChannelCount     = 2
	EnetdefaultThrottleInterval = 5000 // ms
	EnetdefaultThrottleAcce     = 2
	EnetdefaultThrottleDece     = 2
	EnetdefaultTickMs           = 20  // ms
	EnetdefaultRtt              = 500 //ms
	EnetudpSize                 = 65536
	EnetdefaultThrottle         = 32
	EnetthrottleScale           = 32
	EnettimeoutLimit            = 32   // 30 seconds
	EnettimeoutMin              = 5000 // 5 second
	EnettimeoutMax              = 30000
	EnetpingInterval            = 1000 // 1 second
	EnetbpsInterval             = 1000 // 1 second
)
const (
	ChannelIDNone uint8 = 0xff
	ChannelIDAll        = 0xfe
)

const (
	EnetpeerConnectResultDuplicated = 1
	EnetpeerDisconnectResultInvalid
)
const EnetpeerIDAny uint32 = 0xffffffff

/* uncompatiable with enet origin protocol
EnetcmdUnsequenced    // +unseq flag
EnetcmdBandwidthlimit // ack flag
EnetcmdThrottle       // ack flag
EnetcmdUnreliableFragment
*/
