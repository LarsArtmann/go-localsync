{
  description = "Go-LocalSync - Generic synchronization SDK with CQRS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    systems.url = "github:nix-systems/default";

    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    go-nix-helpers = {
      url = "git+ssh://git@github.com/LarsArtmann/go-nix-helpers?ref=master";
      flake = false;
    };

    go-cqrs-lite = {
      url = "git+ssh://git@github.com/LarsArtmann/go-cqrs-lite?ref=master";
      flake = false;
    };

    go-branded-id = {
      url = "git+ssh://git@github.com/LarsArtmann/go-branded-id?ref=master";
      flake = false;
    };

    go-error-family = {
      url = "git+ssh://git@github.com/LarsArtmann/go-error-family?ref=master";
      flake = false;
    };
  };

  outputs =
    inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import inputs.systems;

      imports = [ inputs.treefmt-nix.flakeModule ];

      perSystem =
        {
          config,
          pkgs,
          lib,
          ...
        }:
        let
          goTags = [
            "goexperiment.jsonv2"
          ];
          tagFlags = builtins.concatStringsSep " " (map (t: "-tags=${t}") goTags);

          mkPreparedSource = import (inputs.go-nix-helpers + "/mkPreparedSource.nix") {
            inherit pkgs lib;
            goPkg = pkgs.go_1_26;
          };

          preparedSrc = mkPreparedSource {
            name = "go-localsync";
            version = self.rev or self.dirtyRev or "dev";
            src = ./.;
            deps = {
              "github.com/larsartmann/go-cqrs-lite" = inputs.go-cqrs-lite;
              "github.com/larsartmann/go-branded-id" = inputs.go-branded-id;
              "github.com/larsartmann/go-error-family" = inputs.go-error-family;
            };
          };

          vendorHash = "sha256-P6gAklY2vIDyhxng9ajwgXhOyFgaDSZDXCEas/GodS8=";

          modFodAttrs = {
            proxyVendor = false;
            modBuildPhase = ''
              runHook preBuild
              export GOCACHE=$TMPDIR/go-cache
              export GOPATH="$TMPDIR/go"
              cd "$modRoot"
              go mod tidy
              go mod vendor
              mkdir -p vendor
              runHook postBuild
            '';
            modInstallPhase = ''
              cp -r --reflink=auto vendor $out
              cp go.mod $out/go.mod
              cp go.sum $out/go.sum
            '';
            preBuild = ''
              if [ -n "''${goModules:-}" ] && [ -f "$goModules/go.mod" ]; then
                cp "$goModules/go.mod" go.mod
                cp "$goModules/go.sum" go.sum
              fi
              export GOEXPERIMENT=jsonv2
            '';
          };
        in
        {
          packages = {
            default = pkgs.buildGoModule {
              pname = "go-localsync";
              version = self.rev or self.dirtyRev or "dev";
              src = preparedSrc;
              inherit vendorHash;
              inherit (modFodAttrs)
                proxyVendor
                modBuildPhase
                modInstallPhase
                preBuild
                ;
              meta = with pkgs.lib; {
                description = "Generic synchronization SDK with CQRS";
                homepage = "https://github.com/larsartmann/go-localsync";
                license = licenses.mit;
                maintainers = [
                  {
                    name = "Lars Artmann";
                    github = "LarsArtmann";
                  }
                ];
              };
            };

            cqrs-lint = pkgs.buildGoModule {
              pname = "cqrs-lint";
              version = self.rev or self.dirtyRev or "dev";
              src = preparedSrc;
              inherit vendorHash;
              inherit (modFodAttrs)
                proxyVendor
                modBuildPhase
                modInstallPhase
                preBuild
                ;
              subPackages = [ "cmd/cqrs-lint" ];
              meta = with pkgs.lib; {
                description = "Static CQRS architectural-invariant linter";
                homepage = "https://github.com/larsartmann/go-localsync";
                license = licenses.mit;
                mainProgram = "cqrs-lint";
              };
            };
          };

          apps = {
            test = {
              type = "app";
              program = pkgs.writeShellApplication {
                name = "run-test";
                runtimeInputs = [ pkgs.go_1_26 ];
                text = "go test ${tagFlags} -race -v -coverprofile=coverage.out ./...";
              };
            };

            lint = {
              type = "app";
              program = pkgs.writeShellApplication {
                name = "run-lint";
                runtimeInputs = [
                  pkgs.go_1_26
                  pkgs.golangci-lint
                ];
                text = "golangci-lint run ./...";
              };
            };

            cqrs-lint = {
              type = "app";
              program = "${config.packages.cqrs-lint}/bin/cqrs-lint";
            };
          };

          devShells = {
            default = pkgs.mkShell {
              GOFLAGS = tagFlags;
              GOPRIVATE = "github.com/larsartmann/*,github.com/LarsArtmann/*";
              packages = with pkgs; [
                go_1_26
                golangci-lint
                ginkgo
                gotools
                gofumpt
              ];
            };

            ci = pkgs.mkShellNoCC {
              GOFLAGS = tagFlags;
              GOPRIVATE = "github.com/larsartmann/*,github.com/LarsArtmann/*";
              packages = with pkgs; [
                go_1_26
                golangci-lint
              ];
            };
          };

          treefmt = {
            projectRootFile = "go.mod";
            settings.excludes = [
              "vendor/**"
            ];
            programs = {
              gofumpt.enable = true;
              goimports.enable = true;
              nixfmt.enable = true;
            };
          };

          formatter = config.treefmt.build.wrapper;

          checks = {
            format = config.treefmt.build.check self;
            build = config.packages.default;
            test = config.packages.default.overrideAttrs (_: {
              doCheck = true;
            });
            # Architectural gate: fails when pkg/cqrs drifts from ADR-0004.
            cqrs-lint =
              pkgs.runCommand "cqrs-lint-check" { nativeBuildInputs = [ config.packages.cqrs-lint ]; }
                ''
                  cqrs-lint -pkg ${./pkg/cqrs}
                  touch $out
                '';
          };
        };

      flake.overlays.default = final: _prev: {
        go-localsync = self.packages.${final.stdenv.system}.default;
      };

    };
}
