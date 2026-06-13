package boltstore

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// BenchmarkPutHeavyObject models the crystal-veins pattern: repeated attr
// sets on an object carrying ~376 attributes (~88KB encoded).
func BenchmarkPutHeavyObject(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.bolt"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	obj := &gamedb.Object{DBRef: 309, Name: "VeinsHost"}
	for i := 0; i < 376; i++ {
		obj.Attrs = append(obj.Attrs, gamedb.Attribute{
			Number: 256 + i,
			Value:  fmt.Sprintf("vein payload %04d: some moderately long attribute value to mimic the real database", i),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj.Attrs[i%376].Value = fmt.Sprintf("updated %d", i)
		if err := s.PutObject(obj); err != nil {
			b.Fatal(err)
		}
	}
}
