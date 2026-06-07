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


      checks.format = config.treefmt.build.check self;
      checks.build = config.packages.default;
      perSystem =
        { config, pkgs, ... }:
        {
          packages.default = pkgs.buildGoModule {
            pname = "go-localsync";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;
            meta = with pkgs.lib; {
              description = "Generic synchronization SDK with CQRS";
              homepage = "https://github.com/larsartmann/go-localsync";
              license = licenses.mit;
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
            };          };

          treefmt = {
            projectRootFile = "go.mod";
            programs = {
              gofumpt.enable = true;
              goimports.enable = true;
              nixfmt.enable = true;
            };
          };

          formatter = config.treefmt.build.wrapper;
        };

      flake.overlays.default = final: _prev: {
        go-localsync = self.packages.${final.stdenv.system}.default;
      };

    };
}
