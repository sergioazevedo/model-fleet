#!/usr/bin/env bash
set -euo pipefail
set +x

if [[ $# -lt 1 || $# -gt 2 || "$1" == -* ]]; then
  echo "Usage: $0 <ssh-target> [identity-file]" >&2
  exit 2
fi

target=$1
sync_credentials=false
if [[ -n "${GROQ_API_KEY+x}" || -n "${MISTRAL_API_KEY+x}" ]]; then
  if [[ -z "${GROQ_API_KEY:-}" || -z "${MISTRAL_API_KEY:-}" ]]; then
    echo "Set both GROQ_API_KEY and MISTRAL_API_KEY to update remote credentials." >&2
    exit 2
  fi
  groq_key=$GROQ_API_KEY
  mistral_key=$MISTRAL_API_KEY
  unset GROQ_API_KEY MISTRAL_API_KEY
  for key in "$groq_key" "$mistral_key"; do
    if [[ "$key" == *"'"* || "$key" == *$'\n'* || "$key" == *$'\r'* ]]; then
      echo "API keys cannot contain quotes or line breaks in the systemd env file." >&2
      exit 2
    fi
  done
  sync_credentials=true
fi

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
ssh "${ssh_options[@]}" "$target" 'command -v systemctl >/dev/null && command -v curl >/dev/null' || {
  echo "Remote setup is incomplete. See docs/deployment.md for the systemd prerequisites." >&2
  exit 1
}
if [[ "$sync_credentials" == false ]] &&
  ! ssh "${ssh_options[@]}" "$target" 'test -s "$HOME/.config/model-fleet/router.env"'; then
  echo "No provider keys supplied and no remote router.env exists." >&2
  exit 1
fi

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

echo "Building Model Fleet for linux/$goarch..."
(cd "$repo_root" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -o "$build_dir/model-fleet-router" ./cmd/router)

ssh "${ssh_options[@]}" "$target" 'mkdir -p "$HOME/.local/bin" "$HOME/.config/systemd/user" && install -d -m 700 "$HOME/.config/model-fleet" && rm -f "$HOME/.config/model-fleet/router.env.next"'
scp "${ssh_options[@]}" "$build_dir/model-fleet-router" "$target:$remote_home/.local/bin/model-fleet-router.next"
scp "${ssh_options[@]}" "$repo_root/systemd/model-fleet-router.service" "$target:$remote_home/.config/systemd/user/model-fleet-router.service.next"
if [[ "$sync_credentials" == true ]]; then
  printf "GROQ_API_KEY='%s'\nMISTRAL_API_KEY='%s'\n" "$groq_key" "$mistral_key" |
    ssh "${ssh_options[@]}" "$target" 'umask 077; cat > "$HOME/.config/model-fleet/router.env.next"'
  unset groq_key mistral_key
fi

ssh "${ssh_options[@]}" "$target" 'bash -se' <<'REMOTE'
set -euo pipefail

binary="$HOME/.local/bin/model-fleet-router"
unit="$HOME/.config/systemd/user/model-fleet-router.service"
credentials="$HOME/.config/model-fleet/router.env"
credentials_updated=false

rollback() {
  if [[ "$credentials_updated" == true ]]; then
    if [[ -f "$credentials.previous" ]]; then
      mv -f "$credentials.previous" "$credentials"
    else
      rm -f "$credentials"
    fi
  fi
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
if [[ -f "$credentials.next" ]]; then
  if [[ -f "$credentials" ]]; then
    cp -p "$credentials" "$credentials.previous"
  fi
  chmod 600 "$credentials.next"
  mv -f "$credentials.next" "$credentials"
  credentials_updated=true
fi
mv -f "$binary.next" "$binary"
chmod 755 "$binary"
mv -f "$unit.next" "$unit"
if ! systemctl --user daemon-reload ||
  ! systemctl --user enable model-fleet-router.service ||
  ! systemctl --user restart model-fleet-router.service; then
  rollback
  exit 1
fi

for attempt in {1..15}; do
  if systemctl --user is-active --quiet model-fleet-router.service &&
    curl --fail --silent --show-error --max-time 2 http://127.0.0.1:4545/healthz >/dev/null 2>&1; then
    rm -f "$credentials.previous"
    echo "Model Fleet is healthy on 127.0.0.1:4545"
    exit 0
  fi
  sleep 1
done

echo "Model Fleet did not become healthy; restoring the previous binary." >&2
rollback
exit 1
REMOTE
