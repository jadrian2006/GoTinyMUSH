package gamedb

// Well-known (built-in) attribute numbers from attrs.h
// These are system-defined and always present.
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
	21:  "PAY",
	22:  "OPAY",
	23:  "APAY",
	24:  "OLISTEN",
	25:  "AHEAR",
	26:  "LAST",
	27:  "QUEUEMAX",
	28:  "IDESC",
	29:  "ENTER",
	30:  "OXENTER",
	31:  "AENTER",
	32:  "ADESC",
	33:  "ODESC",
	34:  "RQUOTA",
	35:  "ACONNECT",
	36:  "ADISCONNECT",
	37:  "ALLOWANCE",
	38:  "LOCK",
	39:  "NAME",
	40:  "COMMENT",
	41:  "USE",
	42:  "OUSE",
	43:  "SEMAPHORE",
	44:  "TIMEOUT",
	45:  "QUOTA",
	46:  "LEAVE",
	47:  "OLEAVE",
	48:  "ALEAVE",
	49:  "OENTER",
	50:  "OXLEAVE",
	51:  "MOVE",
	52:  "OMOVE",
	53:  "AMOVE",
	54:  "ALIAS",
	55:  "LENTER",
	56:  "LLEAVE",
	57:  "LPAGE",
	58:  "LUSE",
	59:  "LGIVE",
	60:  "EALIAS",
	61:  "LALIAS",
	62:  "EFAIL",
	63:  "OEFAIL",
	64:  "AEFAIL",
	65:  "LFAIL",
	66:  "OLFAIL",
	67:  "ALFAIL",
	68:  "REJECT",
	69:  "AWAY",
	70:  "IDLE",
	71:  "UFAIL",
	72:  "OUFAIL",
	73:  "AUFAIL",
	74:  "PFAIL",
	75:  "TPORT",
	76:  "OTPORT",
	77:  "OXTPORT",
	78:  "ATPORT",
	79:  "PRIVS",
	80:  "LOGINDATA",
	81:  "LTPORT",
	82:  "LDROP",
	83:  "LRECEIVE",
	84:  "LASTSITE",
	85:  "INPREFIX",
	86:  "PREFIX",
	87:  "INFILTER",
	88:  "FILTER",
	89:  "LLINK",
	90:  "LTELOUT",
	91:  "FORWARDLIST",
	92:  "MAILFOLDERS",
	93:  "LUSER",
	94:  "LPARENT",
	95:  "VA",
	96:  "VB",
	97:  "VC",
	98:  "VD",
	99:  "VE",
	100: "VF",
	101: "VG",
	102: "VH",
	103: "VI",
	104: "VJ",
	105: "VK",
	106: "VL",
	107: "VM",
	108: "VN",
	109: "VO",
	110: "VP",
	111: "VQ",
	112: "VR",
	113: "VS",
	114: "VT",
	115: "VU",
	116: "VV",
	117: "VW",
	118: "VX",
	119: "VY",
	120: "VZ",
	129: "LCONTROL",
	200: "SPEECHMOD",
	201: "SPEECHLOCK",
	202: "PROPDIR",
	203: "CREATED_TIME",
	204: "MODIFIED_TIME",
	210: "PROGCMD",
	213: "LASTIP",
	214: "CONFORMAT",  // A_LCON_FMT in C source
	215: "EXITFORMAT", // A_LEXITS_FMT in C source
	218: "LASTSITE",
	// Additional high-number system attrs
	222: "NAMEFORMAT", // A_NAME_FMT in C source
	240: "VRML_URL",
	241: "HTDESC",
	242: "REASON",
	243: "REGINFO",
	244: "CONNINFO",
	252: "DAILYATTRIB",
	253: "LCHOWN",
	// A_USER_START marks where user-defined attrs begin
}

// Well-known attribute number constants.
const A_PROGCMD = 210

// A_USER_START is the first attribute number available for user-defined attrs.
const A_USER_START = 256

// WellKnownAttrFlags maps built-in attribute numbers to their default flags.
// Matches the attr flag definitions in C TinyMUSH's attrs.h.
var WellKnownAttrFlags = map[int]int{
	5:   AFDark | AFInternal,             // A_PASS — password hash
	26:  AFInternal,                       // A_LAST — last command (internal)
	34:  AFInternal | AFGod,               // A_RQUOTA
	37:  AFInternal | AFGod,               // A_ALLOWANCE
	38:  AFInternal | AFIsLock,            // A_LOCK — default lock
	39:  AFInternal,                       // A_NAME
	43:  AFInternal,                       // A_SEMAPHORE
	44:  AFInternal,                       // A_TIMEOUT
	45:  AFInternal | AFGod,               // A_QUOTA
	55:  AFInternal | AFIsLock,            // A_LENTER
	56:  AFInternal | AFIsLock,            // A_LLEAVE
	57:  AFInternal | AFIsLock,            // A_LPAGE
	58:  AFInternal | AFIsLock,            // A_LUSE
	59:  AFInternal | AFIsLock,            // A_LGIVE
	79:  AFDark | AFNoCMD | AFInternal,    // A_PRIVS
	80:  AFDark | AFNoCMD | AFInternal,    // A_LOGINDATA
	81:  AFInternal | AFIsLock,            // A_LTPORT
	82:  AFInternal | AFIsLock,            // A_LDROP
	83:  AFInternal | AFIsLock,            // A_LRECEIVE
	84:  AFDark | AFNoCMD | AFInternal | AFGod, // A_LASTSITE
	89:  AFInternal | AFIsLock,            // A_LLINK
	90:  AFInternal | AFIsLock,            // A_LTELOUT
	92:  AFInternal,                       // A_MAILFOLDERS
	93:  AFInternal | AFIsLock,            // A_LUSER
	94:  AFInternal | AFIsLock,            // A_LPARENT
	129: AFInternal | AFIsLock,            // A_LCONTROL
	210: AFInternal | AFDark,                       // A_PROGCMD
	213: AFDark | AFNoCMD | AFInternal | AFGod, // A_LASTIP
}
