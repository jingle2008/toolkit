//go:build !unix

package configloader

// lockFile is a no-op on non-unix platforms, which lack flock. Each
// update still re-reads the file before saving, so only writes racing
// within that short window can be lost — acceptable for a tool
// primarily targeting unix.
func lockFile(_ string) (func(), error) {
	return func() {}, nil
}
