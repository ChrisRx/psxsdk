package yaroze

import (
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/ChrisRx/psxsdk/pkg/format/ecoff"
	"github.com/ChrisRx/psxsdk/pkg/format/psx"
)

func TestYarozeConvertEcoffToEXE(t *testing.T) {
	input, err := ecoff.Open("../format/ecoff/testdata/main-ecoff")
	if err != nil {
		t.Fatal(err)
	}

	exe := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:     psx.ExecutableSignature,
			PC0:       input.Entry,
			TextAddr:  input.Entry,
			TextSize:  input.Size(),
			StackAddr: 0x801fff00,
		},
		Sections: []*psx.Section{
			&psx.Section{
				Name: "text",
				Addr: 0x80010000,
				Data: input.Data(),
			},
		},
	}

	if err := PatchExecutable(exe); err != nil {
		t.Fatal(err)
	}

	psx.AlignTextData(exe, 2048)

	exe, err = Combine(exe)
	if err != nil {
		t.Fatal(err)
	}
	expected := "36174751559112FBF3E6255BE181C9FD"
	sum := fmt.Sprintf("%X", md5.Sum(exe.Bytes()))
	if sum != expected {
		t.Fatalf("expected md5 %s, received %s", expected, sum)
	}
}
