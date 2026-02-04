{
  description = "G2 - Smart Git merge with semantic conflict resolution";

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
        packages = {
          default = pkgs.buildGoModule {
            pname = "g2";
            version = "0.1.0";

            src = ./.;

            # Hash for Go module dependencies
            vendorHash = "sha256-tiPnv/kMN7tQcyFr/SrxQeFxHJGtT6pFUH5t+k4w0aM=";

            # Git is needed for tests
            nativeCheckInputs = [ pkgs.git ];

            meta = with pkgs.lib; {
              description = "Smart Git merge with semantic conflict resolution";
              homepage = "https://github.com/simonkoeck/g2";
              license = licenses.mit;
              maintainers = [ ];
              mainProgram = "g2";
            };
          };

          g2 = self.packages.${system}.default;
        };

        apps = {
          default = flake-utils.lib.mkApp {
            drv = self.packages.${system}.default;
          };
          g2 = self.apps.${system}.default;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            git
          ];
        };
      }
    );
}
