// The in-process linker, as TinyGo's builder/lld.cpp provides it, with only
// the ELF driver registered: the PlayStation is the one target this build
// links for, and each of the other flavors would bring its library in.
#include <lld/Common/Driver.h>

LLD_HAS_DRIVER(elf)

extern "C" {

bool tinygo_link(int argc, char **argv) {
	std::vector<const char*> args(argv, argv + argc);
	lld::Result r = lld::lldMain(args, llvm::outs(), llvm::errs(), {{lld::Gnu, &lld::elf::link}});
	return !r.retCode;
}

} // extern "C"
