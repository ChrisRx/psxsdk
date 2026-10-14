{
  description = "Tools and libraries for Sony PlayStation 1 development";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);

      # The Net Yaroze SDK targets 32-bit little-endian MIPS with no operating
      # system. mipsel-none-elf is the modern spelling of that target and is
      # still supported by current GCC and Binutils, unlike the mipsel-ecoff
      # target the SDK originally shipped for.
      #
      # The gcc attributes are not cosmetic. nixpkgs defaults this target to
      # mips32r2 with hardware floating point, and because cc-wrapper injects
      # -march= from gcc.arch, every compile arrives with -march=mips32r2
      # already applied. That is not just a wrong default to override on the
      # command line: GCC rejects the combination outright with "-mips32r2
      # conflicts with the other architecture options, which specify a mips1
      # processor". Setting them here also builds libgcc itself for MIPS I
      # without an FPU, which is what the PlayStation's R3000A needs.
      crossSystem = {
        config = "mipsel-none-elf";
        libc = "newlib";
        gcc = {
          arch = "mips1";
          float = "soft";
        };
      };

      # gccWithoutTargetLibc is the stage of the cross bootstrap that exists
      # before a target C library does, and it is the right stage here: the C
      # library for this platform is libps, which is already resident in the
      # console's RAM and is reached through the fixed addresses. Nothing ever
      # links newlib, and depending on it only adds a build that has to
      # succeed, which its MIPS libgloss board-support code does not do for
      # MIPS I.
      crossToolchain = cross: [
        cross.buildPackages.gccWithoutTargetLibc
        cross.buildPackages.binutils-unwrapped
      ];

      # TinyGo, from a fork carrying the playstation target.
      #
      # It has to be a fork rather than an out-of-tree target file, because
      # TinyGo selects a target's runtime files by build tag from inside its
      # own source tree. The three files the PlayStation needs there are
      # src/runtime/runtime_playstation.go, src/runtime/arch_playstation.go
      # and src/runtime/interrupt/interrupt_playstation.go; the alternative
      # was for every build to stage a modified copy of TINYGOROOT.
      #
      # This is nixpkgs' own derivation with the source swapped, so it keeps
      # the LLVM and Go versions nixpkgs pinned for the release it packaged,
      # which is what the fork is branched from. It does mean building TinyGo
      # rather than taking it from the binary cache.
      #
      # To move to a newer commit of the fork: change rev, set hash and
      # vendorHash to lib.fakeHash, build, and take the values nix reports.
      mkTinygo =
        pkgs:
        pkgs.tinygo.overrideAttrs (
          final: prev: {
            version = "0.44.0-dev-playstation";

            src = pkgs.fetchFromGitHub {
              owner = "ChrisRx";
              repo = "tinygo";
              rev = "140a09560423413ee521416f19c5a5a007449a54";
              hash = "sha256-JEZ1mIa1vLFOKLdNLY+Cl4GDbwzN0uwnhb8fvice9Mg=";
              fetchSubmodules = true;
              # cmsis-svd carries several GB of vendor data for parts that have
              # nothing to do with this, which nixpkgs drops for the same reason.
              postFetch = ''
                rm -r $out/lib/cmsis-svd/data/{SiliconLabs,Freescale}
              '';
            };

            vendorHash = "sha256-Bliaz1fMIUnHD/S3ggNMRXh0rIlwrCCOXMr+uiP8NGc=";

            # nixpkgs carries a patch for the makefiles, which is against the
            # release it packaged and no longer applies: make/release.mk has
            # moved on. The same changes are made here as substitutions, which
            # do not care where in the file the lines are, and which fail loudly
            # rather than silently if the fork moves again.
            #
            # All three are about Nix supplying something the makefile would
            # otherwise go looking for. LLVM_PROJECTDIR defaults to an
            # llvm-project checkout that is not part of this source tree, so the
            # copies out of it have to come from the store instead; the release
            # rule's prerequisites would rebuild what the derivation's own phases
            # already built; and brew is not a thing that exists here.
            patches = [ ];
            postPatch = ''
              substituteInPlace make/config.mk \
                --replace-fail 'BREW_PREFIX := $(shell brew --prefix)' 'BREW_PREFIX :='

              substituteInPlace make/release.mk \
                --replace-fail 'build/release: tinygo gen-device $(if $(filter 1,$(USE_SYSTEM_BINARYEN)),,binaryen)' 'build/release:' \
                --replace-fail '@cp -p $(abspath $(CLANG_SRC))/lib/Headers/*.h build/release/tinygo/lib/clang/include' '@true' \
                --replace-fail '@cp -rp ''${LLVM_PROJECTDIR}/compiler-rt/lib/builtins build/release/tinygo/lib/compiler-rt-builtins' '@cp -rp lib/compiler-rt-builtins build/release/tinygo/lib/compiler-rt-builtins' \
                --replace-fail '@cp -rp ''${LLVM_PROJECTDIR}/compiler-rt/LICENSE.TXT  build/release/tinygo/lib/compiler-rt-builtins' '@true'
            ''
            + prev.postPatch;

            # "make smoketest" builds a program for every board TinyGo supports,
            # which is a long time to spend on a check that says nothing about
            # this target. The real check is "make -C elf tinygo-check".
            doCheck = false;
          }
        );
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        rec {
          default = psxsdk;

          psxsdk = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
            pname = "psxsdk";
            version = "0.2.0";
            src = ./.;

            vendorHash = "sha256-0FVOyrxTVvYVvEKKfcnUxxjy5MoU6LuxDk7LAVHJBwo=";

            # Matches the Makefile, which trims the build directory out of
            # recorded paths.
            ldflags = [
              "-s"
              "-w"
            ];

            meta = {
              description = "Tools and libraries for PlayStation 1 development";
              homepage = "https://github.com/ChrisRx/psxsdk";
              license = pkgs.lib.licenses.mit;
              mainProgram = "eco2exe";
            };
          };

        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          cross = import nixpkgs {
            inherit system;
            inherit crossSystem;
          };
        in
        {
          default = pkgs.mkShell {
            packages = crossToolchain cross ++ [
              pkgs.go_1_27
              pkgs.gnumake
              pkgs.go-task
              (mkTinygo pkgs)
              pkgs.mednafen
            ];
          };

          # What CI needs to run "task test", and nothing else. The default
          # shell builds TinyGo from source, which is a long time to spend on
          # a run that only compiles and tests host Go code.
          ci = pkgs.mkShell {
            packages = [
              pkgs.go_1_27
              pkgs.go-task
            ];
          };
        }
      );

      formatter = forAllSystems (system: (import nixpkgs { inherit system; }).nixpkgs-fmt);
    };
}
