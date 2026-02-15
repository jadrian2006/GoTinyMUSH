package gamedb

// Well-known (built-in) attribute numbers from constants.h
// These are system-defined and always present.
// Numbers MUST match the C TinyMUSH source exactly since the flatfile uses these numbers.
var WellKnownAttrs = map[int]string{
	1:   "OSUCC",
	2:   "OFAIL",
	3:   "FAIL",
	4:   "SUCC",
	5:   "PASS",
	6:   "DESC",
	7:   "SEX",
	8:   "ODROP",
	9:   "DROP",
	10:  "OKILL",
	11:  "KILL",
	12:  "ASUCC",
	13:  "AFAIL",
	14:  "ADROP",
	15:  "AKILL",
	16:  "AUSE",
	17:  "CHARGES",
	18:  "RUNOUT",
	19:  "STARTUP",
	20:  "ACLONE",
	21:  "APAY",
	22:  "OPAY",
	23:  "PAY",
	24:  "COST",
	25:  "MONEY",
	26:  "LISTEN",
	27:  "AAHEAR",
	28:  "AMHEAR",
	29:  "AHEAR",
	30:  "LAST",
	31:  "QUEUEMAX",
	32:  "IDESC",
	33:  "ENTER",
	34:  "OXENTER",
	35:  "AENTER",
	36:  "ADESC",
	37:  "ODESC",
	38:  "RQUOTA",
	39:  "ACONNECT",
	40:  "ADISCONNECT",
	41:  "ALLOWANCE",
	42:  "LOCK",
	43:  "NAME",
	44:  "COMMENT",
	45:  "USE",
	46:  "OUSE",
	47:  "SEMAPHORE",
	48:  "TIMEOUT",
	49:  "QUOTA",
	50:  "LEAVE",
	51:  "OLEAVE",
	52:  "ALEAVE",
	53:  "OENTER",
	54:  "OXLEAVE",
	55:  "MOVE",
	56:  "OMOVE",
	57:  "AMOVE",
	58:  "ALIAS",
	59:  "LENTER",
	60:  "LLEAVE",
	61:  "LPAGE",
	62:  "LUSE",
	63:  "LGIVE",
	64:  "EALIAS",
	65:  "LALIAS",
	66:  "EFAIL",
	67:  "OEFAIL",
	68:  "AEFAIL",
	69:  "LFAIL",
	70:  "OLFAIL",
	71:  "ALFAIL",
	72:  "REJECT",
	73:  "AWAY",
	74:  "IDLE",
	75:  "UFAIL",
	76:  "OUFAIL",
	77:  "AUFAIL",
	// 78: unused (formerly A_PFAIL)
	79:  "TPORT",
	80:  "OTPORT",
	81:  "OXTPORT",
	82:  "ATPORT",
	// 83: unused (formerly A_PRIVS)
	84:  "LOGINDATA",
	85:  "LTPORT",
	86:  "LDROP",
	87:  "LRECEIVE",
	88:  "LASTSITE",
	89:  "INPREFIX",
	90:  "PREFIX",
	91:  "INFILTER",
	92:  "FILTER",
	93:  "LLINK",
	94:  "LTELOUT",
	95:  "FORWARDLIST",
	96:  "MAILFOLDERS",
	97:  "LUSER",
	98:  "LPARENT",
	99:  "LCONTROL",
	100: "VA",
	101: "VB",
	102: "VC",
	103: "VD",
	104: "VE",
	105: "VF",
	106: "VG",
	107: "VH",
	108: "VI",
	109: "VJ",
	110: "VK",
	111: "VL",
	112: "VM",
	113: "VN",
	114: "VO",
	115: "VP",
	116: "VQ",
	117: "VR",
	118: "VS",
	119: "VT",
	120: "VU",
	121: "VV",
	122: "VW",
	123: "VX",
	124: "VY",
	125: "VZ",
	// 126-128: unused
	129: "GFAIL",
	130: "OGFAIL",
	131: "AGFAIL",
	132: "RFAIL",
	133: "ORFAIL",
	134: "ARFAIL",
	135: "DFAIL",
	136: "ODFAIL",
	137: "ADFAIL",
	138: "TFAIL",
	139: "OTFAIL",
	140: "ATFAIL",
	141: "TOFAIL",
	142: "OTOFAIL",
	143: "ATOFAIL",
	144: "LOPEN",
	// High-number system attrs
	202: "AMAIL",
	204: "DAILYATTRIB",
	214: "CONFORMAT",  // A_LCON_FMT
	215: "EXITFORMAT", // A_LEXITS_FMT
	218: "LASTIP",
	221: "HTDESC",
	222: "NAMEFORMAT", // A_NAME_FMT
	231: "PROPDIR",
}

// Well-known attribute number constants.
const A_PROGCMD = 210

// A_USER_START is the first attribute number available for user-defined attrs.
const A_USER_START = 256

// WellKnownAttrFlags maps built-in attribute numbers to their default flags.
// Matches the attr flag definitions in C TinyMUSH's attrs.h.
var WellKnownAttrFlags = map[int]int{
	5:   AFDark | AFInternal,                       // A_PASS — password hash
	25:  AFInternal,                                 // A_MONEY
	30:  AFInternal,                                 // A_LAST — last login time
	38:  AFInternal | AFGod,                         // A_RQUOTA
	41:  AFInternal | AFGod,                         // A_ALLOWANCE
	42:  AFInternal | AFIsLock,                      // A_LOCK — default lock
	43:  AFInternal,                                 // A_NAME
	47:  AFInternal,                                 // A_SEMAPHORE
	48:  AFInternal,                                 // A_TIMEOUT
	49:  AFInternal | AFGod,                         // A_QUOTA
	59:  AFInternal | AFIsLock,                      // A_LENTER
	60:  AFInternal | AFIsLock,                      // A_LLEAVE
	61:  AFInternal | AFIsLock,                      // A_LPAGE
	62:  AFInternal | AFIsLock,                      // A_LUSE
	63:  AFInternal | AFIsLock,                      // A_LGIVE
	84:  AFDark | AFNoCMD | AFInternal,              // A_LOGINDATA
	85:  AFInternal | AFIsLock,                      // A_LTPORT
	86:  AFInternal | AFIsLock,                      // A_LDROP
	87:  AFInternal | AFIsLock,                      // A_LRECEIVE
	88:  AFDark | AFNoCMD | AFInternal | AFGod,      // A_LASTSITE
	93:  AFInternal | AFIsLock,                      // A_LLINK
	94:  AFInternal | AFIsLock,                      // A_LTELOUT
	96:  AFInternal,                                 // A_MAILFOLDERS
	97:  AFInternal | AFIsLock,                      // A_LUSER
	98:  AFInternal | AFIsLock,                      // A_LPARENT
	99:  AFInternal | AFIsLock,                      // A_LCONTROL
	210: AFInternal | AFDark,                        // A_PROGCMD
	218: AFDark | AFNoCMD | AFInternal | AFGod,      // A_LASTIP
}
