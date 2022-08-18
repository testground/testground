{
  description = "Placebo test plan built with nix";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/release-22.05";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { system = system; };
      in
      {
        packages.placebo = pkgs.buildGo117Module {
          src = ./.;
          pname = "placebo";
          version = "0.0.1";
          # vendorSha256 = pkgs.lib.fakeSha256;
          vendorSha256 = "sha256-DmUScLup1RxRmKyRH44WMnBYEho94XAUufMyDK3OMjo=";
        };
        defaultPackage = self.packages.${system}.placebo;
      });
}
