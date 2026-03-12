package gamedb

// Well-known (built-in) attribute numbers from constants.h
// These are system-defined and always present.
// Numbers MUST match the C TinyMUSH source exactly since the flatfile uses these numbers.
// Names MUST match C TinyMUSH's casing (from db.c attr table) for display compatibility.
var WellKnownAttrs = map[int]string{
	1:   "Osucc",
	2:   "Ofail",
	3:   "Fail",
	4:   "Succ",
	5:   "Pass",
	6:   "Desc",
	7:   "Sex",
	8:   "Odrop",
	9:   "Drop",
	10:  "Okill",
	11:  "Kill",
	12:  "Asucc",
	13:  "Afail",
	14:  "Adrop",
	15:  "Akill",
	16:  "Ause",
	17:  "Charges",
	18:  "Runout",
	19:  "Startup",
	20:  "Aclone",
	21:  "Apay",
	22:  "Opay",
	23:  "Pay",
	24:  "Cost",
	25:  "Money",
	26:  "Listen",
	27:  "Aahear",
	28:  "Amhear",
	29:  "Ahear",
	30:  "Last",
	31:  "Queuemax",
	32:  "Idesc",
	33:  "Enter",
	34:  "Oxenter",
	35:  "Aenter",
	36:  "Adesc",
	37:  "Odesc",
	38:  "Rquota",
	39:  "Aconnect",
	40:  "Adisconnect",
	41:  "Allowance",
	42:  "DefaultLock",
	43:  "Name",
	44:  "Comment",
	45:  "Use",
	46:  "Ouse",
	47:  "Semaphore",
	48:  "Timeout",
	49:  "Quota",
	50:  "Leave",
	51:  "Oleave",
	52:  "Aleave",
	53:  "Oenter",
	54:  "Oxleave",
	55:  "Move",
	56:  "Omove",
	57:  "Amove",
	58:  "Alias",
	59:  "EnterLock",
	60:  "LeaveLock",
	61:  "PageLock",
	62:  "UseLock",
	63:  "GiveLock",
	64:  "Ealias",
	65:  "Lalias",
	66:  "Efail",
	67:  "Oefail",
	68:  "Aefail",
	69:  "Lfail",
	70:  "Olfail",
	71:  "Alfail",
	72:  "Reject",
	73:  "Away",
	74:  "Idle",
	75:  "Ufail",
	76:  "Oufail",
	77:  "Aufail",
	// 78: unused (formerly A_PFAIL)
	79:  "Tport",
	80:  "Otport",
	81:  "Oxtport",
	82:  "Atport",
	// 83: unused (formerly A_PRIVS)
	84:  "Logindata",
	85:  "TportLock",
	86:  "DropLock",
	87:  "ReceiveLock",
	88:  "Lastsite",
	89:  "Inprefix",
	90:  "Prefix",
	91:  "Infilter",
	92:  "Filter",
	93:  "LinkLock",
	94:  "TeloutLock",
	95:  "Forwardlist",
	96:  "Mailfolders",
	97:  "UserLock",
	98:  "ParentLock",
	99:  "ControlLock",
	100: "Va",
	101: "Vb",
	102: "Vc",
	103: "Vd",
	104: "Ve",
	105: "Vf",
	106: "Vg",
	107: "Vh",
	108: "Vi",
	109: "Vj",
	110: "Vk",
	111: "Vl",
	112: "Vm",
	113: "Vn",
	114: "Vo",
	115: "Vp",
	116: "Vq",
	117: "Vr",
	118: "Vs",
	119: "Vt",
	120: "Vu",
	121: "Vv",
	122: "Vw",
	123: "Vx",
	124: "Vy",
	125: "Vz",
	// 126-128: unused
	129: "Gfail",
	130: "Ogfail",
	131: "Agfail",
	132: "Rfail",
	133: "Orfail",
	134: "Arfail",
	135: "Dfail",
	136: "Odfail",
	137: "Adfail",
	138: "Tfail",
	139: "Otfail",
	140: "Atfail",
	141: "Tofail",
	142: "Otofail",
	143: "Atofail",
	144: "OpenLock",
	// High-number system attrs
	202: "Amail",
	204: "DAILYATTRIB",
	209: "SpeechLock",
	214: "Conformat",   // A_LCON_FMT
	215: "Exitformat",  // A_LEXITS_FMT
	217: "ChownLock",
	218: "Lastip",
	219: "DarkLock",
	221: "Htdesc",
	222: "Nameformat",  // A_NAME_FMT
	223: "KnownLock",
	224: "HeardLock",
	225: "MovedLock",
	226: "KnowsLock",
	227: "HearsLock",
	228: "MovesLock",
	231: "Propdir",
	232: "ROOMFORMAT",
	233: "SMELL",
	234: "OSMELL",
	235: "ASMELL",
	236: "TOUCH",
	237: "OTOUCH",
	238: "ATOUCH",
	239: "TASTE",
	240: "OTASTE",
	241: "ATASTE",
	242: "SOUND",
	243: "OSOUND",
	244: "ASOUND",
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
	30:  AFVisual | AFWizard | AFNoCMD | AFNoProg | AFNoClone, // A_LAST — last login time
	38:  AFInternal | AFGod,                         // A_RQUOTA
	41:  AFInternal | AFGod,                         // A_ALLOWANCE
	42:  AFInternal | AFNoProg | AFNoCMD | AFIsLock,  // A_LOCK — default lock (shown via Key: line)
	43:  AFInternal,                                 // A_NAME
	47:  AFNoProg | AFWizard | AFNoCMD | AFNoClone,   // A_SEMAPHORE
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
	88:  AFNoProg | AFNoCMD | AFNoClone | AFGod,      // A_LASTSITE
	93:  AFNoProg | AFNoCMD | AFIsLock,               // A_LLINK — LinkLock
	94:  AFNoProg | AFNoCMD | AFIsLock,               // A_LTELOUT — TeloutLock
	96:  AFInternal,                                 // A_MAILFOLDERS
	97:  AFNoProg | AFNoCMD | AFIsLock,               // A_LUSER — UserLock
	98:  AFNoProg | AFNoCMD | AFIsLock,               // A_LPARENT — ParentLock
	99:  AFNoProg | AFNoCMD | AFIsLock,               // A_LCONTROL — ControlLock
	209: AFNoProg | AFNoCMD | AFIsLock,               // A_LSPEECH — SpeechLock
	210: AFInternal | AFDark,                        // A_PROGCMD
	217: AFNoProg | AFNoCMD | AFIsLock,               // A_LCHOWN — ChownLock
	218: AFNoProg | AFNoCMD | AFNoClone | AFGod,      // A_LASTIP
	219: AFNoProg | AFNoCMD | AFIsLock,               // A_LDARK — DarkLock
	223: AFNoProg | AFNoCMD | AFIsLock,               // A_LKNOWN — KnownLock
	224: AFNoProg | AFNoCMD | AFIsLock,               // A_LHEARD — HeardLock
	225: AFNoProg | AFNoCMD | AFIsLock,               // A_LMOVED — MovedLock
	226: AFNoProg | AFNoCMD | AFIsLock,               // A_LKNOWS — KnowsLock
	227: AFNoProg | AFNoCMD | AFIsLock,               // A_LHEARS — HearsLock
	228: AFNoProg | AFNoCMD | AFIsLock,               // A_LMOVES — MovesLock
}
