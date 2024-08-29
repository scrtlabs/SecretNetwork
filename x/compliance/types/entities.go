package types

import (
	"encoding/binary"
)

func (vt VerificationType) ToBytes() []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(vt))
	return bytes
}
