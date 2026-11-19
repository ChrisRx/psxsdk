//go:build !irqstamp

package libps

// irqStamp is irqstamp.go's stamp, empty without the irqstamp tag: the
// shipped library makes none.
//
//go:inline
func irqStamp(i int) {}
