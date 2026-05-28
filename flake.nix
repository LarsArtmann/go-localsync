{
  description = "Go-LocalSync - Generic synchronization SDK with CQRS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
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

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_26
            golangci-lint
            ginkgo
            gotools
            gofumpt
          ];
        };
      });
}
