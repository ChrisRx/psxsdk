//go:build !irqstamp

package cleanlib

// irqStamp is irqstamp.go's stamp, empty without the irqstamp tag: the
// shipped library makes none.
//
//go:inline
func irqStamp(i int) {}
