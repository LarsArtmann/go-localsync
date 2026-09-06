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
        vendorHash = "sha256-ZCX5pAXML5c+rKjqFNvk1C4VM+lqxzWN50sTShl0E2A=";
        description = "Generic synchronization SDK with CQRS";

        # One-command full suite: `nix flake check` now runs build + format +
        # hermetic lint (golangci-lint) + hermetic test + the localsync-lint
        # architectural gate. Race stays in CI/local (-race needs cgo).
        lintAsCheck = true;
        enableTestCheck = true;

        extraBuildAttrs.preBuild = "export GOEXPERIMENT=jsonv2";

        shellExtraEnv = {
          GOFLAGS = "-tags=goexperiment.jsonv2";
          GOEXPERIMENT = "jsonv2";
        };

        devShellExtraPackages = pkgs: [
          pkgs.actionlint
          pkgs.dprint
          pkgs.ginkgo
          pkgs.gotools
          pkgs.gofumpt
        ];

        packages = {
          localsync-lint = {
            subPackages = [ "cmd/localsync-lint" ];
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
          checks.localsync-lint =
            pkgs.runCommand "localsync-lint-check"
              {
                nativeBuildInputs = [ config.packages.localsync-lint ];
              }
              ''
                localsync-lint -pkg ${./pkg/cqrs}
                touch $out
              '';
        };
    };
}
