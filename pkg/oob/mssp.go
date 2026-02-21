package oob

// EncodeMSSP builds an MSSP telnet subnegotiation sequence from key-value pairs.
// Format: IAC SB 70 VAR "key" VAL "value" ... IAC SE
// VAR and VAL use the same byte values as MSDP (1 and 2).
func EncodeMSSP(data map[string]string) []byte {
	buf := []byte{IAC, SB, TeloptMSSP}
	for k, v := range data {
		buf = append(buf, MSDPVar)
		buf = append(buf, []byte(k)...)
		buf = append(buf, MSDPVal)
		buf = append(buf, []byte(v)...)
	}
	buf = append(buf, IAC, SE)
	return buf
}
