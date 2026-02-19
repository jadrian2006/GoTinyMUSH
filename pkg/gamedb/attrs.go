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
	42:  "DefaultLock",
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
	59:  "EnterLock",
	60:  "LeaveLock",
	61:  "PageLock",
	62:  "UseLock",
	63:  "GiveLock",
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
	85:  "TportLock",
	86:  "DropLock",
	87:  "ReceiveLock",
	88:  "LASTSITE",
	89:  "INPREFIX",
	90:  "PREFIX",
	91:  "INFILTER",
	92:  "FILTER",
	93:  "LinkLock",
	94:  "TeloutLock",
	95:  "FORWARDLIST",
	96:  "MAILFOLDERS",
	97:  "UserLock",
	98:  "ParentLock",
	99:  "ControlLock",
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
	144: "OpenLock",
	// High-number system attrs
	202: "AMAIL",
	204: "DAILYATTRIB",
	209: "SpeechLock",
	214: "CONFORMAT",  // A_LCON_FMT
	215: "EXITFORMAT", // A_LEXITS_FMT
	217: "ChownLock",
	218: "LASTIP",
	219: "DarkLock",
	221: "HTDESC",
	222: "NAMEFORMAT", // A_NAME_FMT
	223: "KnownLock",
	224: "HeardLock",
	225: "MovedLock",
	226: "KnowsLock",
	227: "HearsLock",
	228: "MovesLock",
	231: "PROPDIR",
}

// Well-known attribute number constants.
const A_SEMAPHORE = 47
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
	42:  AFInternal | AFNoProg | AFNoCMD | AFIsLock,  // A_LOCK — default lock (shown via Key: line)
	43:  AFInternal,                                 // A_NAME
	47:  AFInternal,                                 // A_SEMAPHORE
	48:  AFInternal,                                 // A_TIMEOUT
	49:  AFInternal | AFGod,                         // A_QUOTA
	59:  AFNoProg | AFNoCMD | AFIsLock,               // A_LENTER — EnterLock
	60:  AFNoProg | AFNoCMD | AFIsLock,               // A_LLEAVE — LeaveLock
	61:  AFNoProg | AFNoCMD | AFIsLock,               // A_LPAGE — PageLock
	62:  AFNoProg | AFNoCMD | AFIsLock,               // A_LUSE — UseLock
	63:  AFNoProg | AFNoCMD | AFIsLock,               // A_LGIVE — GiveLock
	84:  AFDark | AFNoCMD | AFInternal,              // A_LOGINDATA
	85:  AFNoProg | AFNoCMD | AFIsLock,               // A_LTPORT — TportLock
	86:  AFNoProg | AFNoCMD | AFIsLock,               // A_LDROP — DropLock
	87:  AFNoProg | AFNoCMD | AFIsLock,               // A_LRECEIVE — ReceiveLock
	88:  AFDark | AFNoCMD | AFInternal | AFGod,      // A_LASTSITE
	93:  AFNoProg | AFNoCMD | AFIsLock,               // A_LLINK — LinkLock
	94:  AFNoProg | AFNoCMD | AFIsLock,               // A_LTELOUT — TeloutLock
	96:  AFInternal,                                 // A_MAILFOLDERS
	97:  AFNoProg | AFNoCMD | AFIsLock,               // A_LUSER — UserLock
	98:  AFNoProg | AFNoCMD | AFIsLock,               // A_LPARENT — ParentLock
	99:  AFNoProg | AFNoCMD | AFIsLock,               // A_LCONTROL — ControlLock
	209: AFNoProg | AFNoCMD | AFIsLock,               // A_LSPEECH — SpeechLock
	210: AFInternal | AFDark,                        // A_PROGCMD
	217: AFNoProg | AFNoCMD | AFIsLock,               // A_LCHOWN — ChownLock
	218: AFDark | AFNoCMD | AFInternal | AFGod,      // A_LASTIP
	219: AFNoProg | AFNoCMD | AFIsLock,               // A_LDARK — DarkLock
	223: AFNoProg | AFNoCMD | AFIsLock,               // A_LKNOWN — KnownLock
	224: AFNoProg | AFNoCMD | AFIsLock,               // A_LHEARD — HeardLock
	225: AFNoProg | AFNoCMD | AFIsLock,               // A_LMOVED — MovedLock
	226: AFNoProg | AFNoCMD | AFIsLock,               // A_LKNOWS — KnowsLock
	227: AFNoProg | AFNoCMD | AFIsLock,               // A_LHEARS — HearsLock
	228: AFNoProg | AFNoCMD | AFIsLock,               // A_LMOVES — MovesLock
}
