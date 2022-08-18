{
  description = "Example test plan built with nix";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/release-22.05";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { system = system; };
        remote-runner-helper = ({ package-name }:
          pkgs.writeScriptBin "remote-runner" ''
            #!${pkgs.bash}/bin/bash

            # Ask our helper where we should be running and what the arch is.
            IFS=, read -r host arch <<< $(${self.packages.${system}.choose-server}/bin/choose-server)
            echo "targeting: " $host $arch

            if [ "$arch" = "x86_64-linux" ]; then
              package="${self.packages.x86_64-linux.${package-name}}"
            elif [ "$host" = "localhost" ]; then
              package="${self.packages.${system}.${package-name}}"
            else
              echo "Arch not setup yet"
              exit 1
            fi

            if [ "$host" = "localhost" ]; then
              echo "Running on localhost"
              $package/bin/${package-name}
            else
              echo "Running on remote"

              # Get the test environment, minus the outputs path. We'll manage that separately
              TEST_ENV=$(env | grep "TEST\|INFLUXDB_URL\|REDIS_HOST\|\SYNC_SERVICE" | grep -v "TEST_OUTPUTS_PATH")

              echo "copying application to $host"
              nix copy -s --to ssh://$host $package

              echo "creating output dir on $host"
              REMOTE_OUTPUTS_PATH=$(ssh $host "mktemp -d")

              # Put the new TEST_OUTPUTS_PATH in the environment
              TEST_ENV=$(printf "%s\nTEST_OUTPUTS_PATH=%s" "$TEST_ENV" "$REMOTE_OUTPUTS_PATH")

              # Run the application on the remote server
              ssh $host -R 5050:localhost:5050 -R 8086:localhost:8086 "env -S \"$TEST_ENV\"  $package/bin/${package-name}"

              # Copy the output of the REMOTE_OUTPUTS_PATH
              ORIGINAL_OUTPUTS_PATH=$(env | grep "TEST_OUTPUTS_PATH" | cut -d "=" -f 2)
              scp -r "$host:$REMOTE_OUTPUTS_PATH/*" "$ORIGINAL_OUTPUTS_PATH"

              # Clean up the outputs on the remote node
              ssh $host rm -r "$REMOTE_OUTPUTS_PATH"
            fi
          '');
      in
      {
        packages.choose-server = pkgs.buildGo117Module {
          src = ./cmd/choose-server;
          pname = "choose-server";
          version = "0.0.1";
          # vendorSha256 = pkgs.lib.fakeSha256;
          vendorSha256 = "sha256-sHPb9jhQgbsgHgzkzMRJRF8nVzVFCqk4dqoD4Tt6SjQ=";
        };
        packages.node = pkgs.buildGo117Module {
          src = ./cmd/node;
          pname = "node";
          version = "0.0.1";
          # vendorSha256 = pkgs.lib.fakeSha256;
          vendorSha256 = "sha256-xM4AXPdTpaDyfuIiSMRQWofxRE4C50F7zP+Llvn5NhA=";
        };
        packages.remote-runner = remote-runner-helper { package-name = "node"; };
      });
}
