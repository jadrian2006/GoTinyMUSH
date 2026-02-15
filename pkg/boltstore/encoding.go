package boltstore

import (
	"bytes"
	"encoding/gob"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func init() {
	gob.Register(gamedb.Object{})
	gob.Register(gamedb.BoolExp{})
	gob.Register(gamedb.Attribute{})
	gob.Register(gamedb.AttrDef{})
	gob.Register(gamedb.Channel{})
	gob.Register(gamedb.ChanAlias{})
	gob.Register(gamedb.StructDef{})
	gob.Register(gamedb.StructInstance{})
	gob.Register(gamedb.MailMessage{})
}

// encodeObject serializes an Object to bytes using gob.
func encodeObject(obj *gamedb.Object) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeObject deserializes bytes back into an Object.
func decodeObject(data []byte) (*gamedb.Object, error) {
	var obj gamedb.Object
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// encodeAttrDef serializes an AttrDef to bytes using gob.
func encodeAttrDef(def *gamedb.AttrDef) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(def); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeAttrDef deserializes bytes back into an AttrDef.
func decodeAttrDef(data []byte) (*gamedb.AttrDef, error) {
	var def gamedb.AttrDef
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&def); err != nil {
		return nil, err
	}
	return &def, nil
}

// encodeChannel serializes a Channel to bytes using gob.
func encodeChannel(ch *gamedb.Channel) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ch); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeChannel deserializes bytes back into a Channel.
func decodeChannel(data []byte) (*gamedb.Channel, error) {
	var ch gamedb.Channel
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// encodeChanAlias serializes a ChanAlias to bytes using gob.
func encodeChanAlias(ca *gamedb.ChanAlias) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ca); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeChanAlias deserializes bytes back into a ChanAlias.
func decodeChanAlias(data []byte) (*gamedb.ChanAlias, error) {
	var ca gamedb.ChanAlias
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&ca); err != nil {
		return nil, err
	}
	return &ca, nil
}

// encodeStructDef serializes a StructDef to bytes using gob.
func encodeStructDef(def *gamedb.StructDef) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(def); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeStructDef deserializes bytes back into a StructDef.
func decodeStructDef(data []byte) (*gamedb.StructDef, error) {
	var def gamedb.StructDef
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&def); err != nil {
		return nil, err
	}
	return &def, nil
}

// encodeStructInst serializes a StructInstance to bytes using gob.
func encodeStructInst(inst *gamedb.StructInstance) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(inst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeStructInst deserializes bytes back into a StructInstance.
func decodeStructInst(data []byte) (*gamedb.StructInstance, error) {
	var inst gamedb.StructInstance
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&inst); err != nil {
		return nil, err
	}
	return &inst, nil
}
