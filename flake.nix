{
  description = "Go-LocalSync - Generic synchronization SDK with CQRS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    go-nix-helpers = {
      url = "git+ssh://git@github.com/LarsArtmann/go-nix-helpers?ref=master";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      go-standard = {
        pname = "go-localsync";
        vendorHash = "sha256-Y2yKOwLbeNwGc+CPGMT6dqjKFq6AsNL0PfV0F4BVF3s=";
        description = "Generic synchronization SDK with CQRS";

        extraBuildAttrs.preBuild = "export GOEXPERIMENT=jsonv2";

        shellExtraEnv = {
          GOFLAGS = "-tags=goexperiment.jsonv2";
          GOEXPERIMENT = "jsonv2";
        };

        devShellExtraPackages = pkgs: [
          pkgs.ginkgo
          pkgs.gotools
          pkgs.gofumpt
        ];

        packages = {
          cqrs-lint = {
            subPackages = [ "cmd/cqrs-lint" ];
            description = "Static CQRS architectural-invariant linter";
          };
        };
      };

      perSystem =
        {
          config,
          pkgs,
          ...
        }:
        {
          # Architectural gate: fails when pkg/cqrs drifts from ADR-0004.
          checks.cqrs-lint =
            pkgs.runCommand "cqrs-lint-check"
              {
                nativeBuildInputs = [ config.packages.cqrs-lint ];
              }
              ''
                cqrs-lint -pkg ${./pkg/cqrs}
                touch $out
              '';
        };
    };
}
