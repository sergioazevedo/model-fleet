#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 || "$1" == -* ]]; then
  echo "Usage: $0 <ssh-target> [identity-file]" >&2
  exit 2
fi

target=$1
ssh_options=(-o BatchMode=yes)
if [[ $# -eq 2 ]]; then
  if [[ ! -r "$2" ]]; then
    echo "SSH identity file is not readable: $2" >&2
    exit 2
  fi
  ssh_options+=(-i "$2" -o IdentitiesOnly=yes)
fi

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
remote_arch=$(ssh "${ssh_options[@]}" "$target" uname -m)
case "$remote_arch" in
  x86_64) goarch=amd64 ;;
  aarch64|arm64) goarch=arm64 ;;
  *) echo "Unsupported remote architecture: $remote_arch" >&2; exit 1 ;;
esac

remote_home=$(ssh "${ssh_options[@]}" "$target" 'printf %s "$HOME"')
ssh "${ssh_options[@]}" "$target" 'test -s "$HOME/.config/model-fleet/router.env" && command -v systemctl >/dev/null && command -v curl >/dev/null' || {
  echo "Remote setup is incomplete. See docs/deployment.md for the credential file and systemd prerequisites." >&2
  exit 1
}

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

echo "Building Model Fleet for linux/$goarch..."
(cd "$repo_root" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -o "$build_dir/model-fleet-router" ./cmd/router)

ssh "${ssh_options[@]}" "$target" 'mkdir -p "$HOME/.local/bin" "$HOME/.config/systemd/user"'
scp "${ssh_options[@]}" "$build_dir/model-fleet-router" "$target:$remote_home/.local/bin/model-fleet-router.next"
scp "${ssh_options[@]}" "$repo_root/deploy/model-fleet-router.service" "$target:$remote_home/.config/systemd/user/model-fleet-router.service.next"

ssh "${ssh_options[@]}" "$target" 'bash -se' <<'REMOTE'
set -euo pipefail

binary="$HOME/.local/bin/model-fleet-router"
unit="$HOME/.config/systemd/user/model-fleet-router.service"

rollback() {
  if [[ -f "$binary.previous" ]]; then
    mv -f "$binary.previous" "$binary"
    systemctl --user restart model-fleet-router.service || true
  else
    systemctl --user disable --now model-fleet-router.service || true
    rm -f "$binary"
  fi
}

if [[ -f "$binary" ]]; then
  cp -p "$binary" "$binary.previous"
fi
mv -f "$binary.next" "$binary"
chmod 755 "$binary"
mv -f "$unit.next" "$unit"
systemctl --user daemon-reload
systemctl --user enable model-fleet-router.service

if ! systemctl --user restart model-fleet-router.service; then
  rollback
  exit 1
fi

for attempt in {1..15}; do
  if systemctl --user is-active --quiet model-fleet-router.service &&
    curl --fail --silent --show-error --max-time 2 http://127.0.0.1:4545/healthz >/dev/null 2>&1; then
    echo "Model Fleet is healthy on 127.0.0.1:4545"
    exit 0
  fi
  sleep 1
done

echo "Model Fleet did not become healthy; restoring the previous binary." >&2
rollback
exit 1
REMOTE
