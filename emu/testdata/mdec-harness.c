// A driver around psxe's MDEC, which made mdec-psxe.txt from the script
// TestMDECMatchesPsxe writes. Build it against psxe 0.10.5-alpha's source,
// as psxe builds itself:
//
//   cc -Ofast -Wno-overflow -I$PSXE -I$PSXE/psx -o mdec-harness \
//       mdec-harness.c $PSXE/psx/dev/mdec.c $PSXE/psx/log.c
//
// and run the test with MDEC_HARNESS pointing at it to write the file again.
//
// The script on stdin is little-endian words. Each is written to the data
// register, except that FFFFFFFFh writes the word after it to the control
// register, and FFFFFFFEh reads as many words as the word after it says from
// the data register, printing each and then the status register.
#include <stdint.h>
#include <stdio.h>

#include "psx/dev/mdec.h"

int main(void) {
    psx_mdec_t *m = psx_mdec_create();
    psx_mdec_init(m);

    uint32_t w;
    while (fread(&w, 4, 1, stdin) == 1) {
        if (w == 0xffffffff) {
            if (fread(&w, 4, 1, stdin) != 1)
                return 1;
            psx_mdec_write32(m, 4, w);
            continue;
        }
        if (w == 0xfffffffe) {
            if (fread(&w, 4, 1, stdin) != 1)
                return 1;
            for (uint32_t i = 0; i < w; i++)
                printf("%08x\n", psx_mdec_read32(m, 0));
            printf("s %08x\n", psx_mdec_read32(m, 4));
            continue;
        }
        psx_mdec_write32(m, 0, w);
    }
    return 0;
}
