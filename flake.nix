{
  description = "Dictée — drop a dictated mp3, get the formatted French text";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      systemOutputs = flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" ] (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          packages = rec {
            dictee = pkgs.buildGoModule {
              pname = "dictee";
              version = "0.1.0";
              src = ./.;
              vendorHash = null;
              nativeBuildInputs = [ pkgs.makeWrapper ];

              # ffmpeg converts the uploaded mp3 to the WAV whisper expects.
              postInstall = ''
                wrapProgram $out/bin/dictee --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.ffmpeg-headless ]}
              '';

              meta = {
                description = "Drop a dictated mp3, get the formatted French text";
                mainProgram = "dictee";
              };
            };
            default = dictee;
          };

          devShells.default = pkgs.mkShell {
            buildInputs = [ pkgs.go pkgs.gopls pkgs.ffmpeg-headless ];
          };
        }
      );

      nixosModule = { config, lib, pkgs, ... }:
        let
          cfg = config.services.dictee;
        in
        {
          options.services.dictee = {
            enable = lib.mkEnableOption "Dictée web service";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
              description = "The Dictée package to use.";
            };

            host = lib.mkOption {
              type = lib.types.str;
              default = "127.0.0.1";
              description = "Address on which Dictée listens.";
            };

            port = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = "HTTP listening port.";
            };

            whisperUrl = lib.mkOption {
              type = lib.types.str;
              default = "http://127.0.0.1:8080";
              description = "Base URL of the whisper-server used for transcription.";
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.dictee = {
              description = "Dictée mp3 transcription";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];

              serviceConfig = {
                ExecStart = "${cfg.package}/bin/dictee -host ${cfg.host} -port ${toString cfg.port} -whisper ${cfg.whisperUrl}";
                Restart = "always";
                RestartSec = 3;
                DynamicUser = true;
                # Hardening: audio only ever lives in a private /tmp.
                PrivateTmp = true;
                PrivateDevices = true;
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                ProtectHome = true;
                ProtectKernelTunables = true;
                ProtectKernelModules = true;
                ProtectControlGroups = true;
                RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
                RestrictNamespaces = true;
                LockPersonality = true;
                MemoryDenyWriteExecute = true;
                SystemCallArchitectures = "native";
              };
            };
          };
        };
    in
    systemOutputs // {
      nixosModules.default = nixosModule;
      nixosModules.dictee = nixosModule;
    };
}
