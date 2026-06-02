{
  description = "VantageOS control plane";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in rec {
          backend = pkgs.stdenv.mkDerivation {
            pname = "vantageos-api";
            version = "0.1.0";
            src = ./backend;
            nativeBuildInputs = [ pkgs.go ];

            buildPhase = ''
              runHook preBuild
              export HOME=$TMPDIR
              export GOCACHE=$TMPDIR/go-cache
              export GOMODCACHE=$TMPDIR/go-mod-cache
              cd "$src"
              go build -trimpath -o "$TMPDIR/vantageos-api" ./cmd/vantageos-api
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              install -Dm755 "$TMPDIR/vantageos-api" "$out/bin/vantageos-api"
              runHook postInstall
            '';

            meta = {
              mainProgram = "vantageos-api";
              description = "VantageOS control-plane API binary";
            };
          };

          frontend = pkgs.stdenv.mkDerivation (finalAttrs: {
            pname = "vantageos-ui";
            version = "0.1.0";
            src = ./.;

            pnpm = pkgs.pnpm_11;
            pnpmDeps = pkgs.fetchPnpmDeps {
              inherit (finalAttrs) pname version src;
              inherit (finalAttrs) pnpm;
              fetcherVersion = 3;
              hash = "sha256-98ykdc2bXu79ozV+TWC5zyHnqDDP9OeWPQ34ZU+hdpQ=";
              pnpmWorkspaces = [ "@vantageos/ui" ];
            };

            nativeBuildInputs = [ pkgs.nodejs_22 pkgs.pnpm_11 pkgs.pnpmConfigHook ];
            pnpmWorkspaces = [ "@vantageos/ui" ];

            buildPhase = ''
              runHook preBuild
              pnpm --filter @vantageos/ui build
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              install -Dm644 frontend/dist/index.html "$out/share/vantageos-ui/index.html"
              cp -r frontend/dist/assets "$out/share/vantageos-ui/"
              runHook postInstall
            '';

            meta = {
              description = "VantageOS control-plane frontend bundle";
            };
          });

          default = backend;
        });

      checks = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in {
          backend-tests = pkgs.stdenv.mkDerivation {
            pname = "vantageos-api-tests";
            version = "0.1.0";
            src = ./backend;
            nativeBuildInputs = [ pkgs.go ];

            buildPhase = ''
              runHook preBuild
              export HOME=$TMPDIR
              export GOCACHE=$TMPDIR/go-cache
              export GOMODCACHE=$TMPDIR/go-mod-cache
              cd "$src"
              go test ./...
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              mkdir -p "$out"
              runHook postInstall
            '';
          };
        });

      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in {
          default = pkgs.mkShell {
            packages = [ pkgs.go pkgs.nodejs_22 pkgs.pnpm ];
          };
        });
    };
}
