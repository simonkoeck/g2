{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go
    pkgs.gcc
    pkgs.gnumake
    pkgs.tree-sitter # Optional: if you need the CLI tool
  ];
}
