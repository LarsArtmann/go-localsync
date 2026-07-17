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
  };

  outputs =
    inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import inputs.systems;

      imports = [ inputs.treefmt-nix.flakeModule ];

      perSystem =
        { config, pkgs, ... }:
        {
          packages = {
            default = pkgs.buildGoModule {
              pname = "go-localsync";
              version = self.rev or self.dirtyRev or "dev";
              src = ./.;
              vendorHash = null;
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

            # Static architectural-invariant linter for pkg/cqrs (ADR-0004).
            cqrs-lint = pkgs.buildGoModule {
              pname = "cqrs-lint";
              version = self.rev or self.dirtyRev or "dev";
              src = ./.;
              vendorHash = null;
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
                text = "go test -race -v -coverprofile=coverage.out ./...";
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
              packages = with pkgs; [
                go_1_26
                golangci-lint
                ginkgo
                gotools
                gofumpt
              ];
            };

            ci = pkgs.mkShellNoCC {
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
            cqrs-lint = pkgs.runCommand "cqrs-lint-check" { nativeBuildInputs = [ config.packages.cqrs-lint ]; } ''
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
