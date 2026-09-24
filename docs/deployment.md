# Deploy Model Fleet

Model Fleet can run on the same Linux host as Meal Planner. The router listens
on `127.0.0.1:4545` under a user-level systemd service, so Meal Planner on that
host can reach it without a public port or reverse proxy. The deploy script
builds a static Go binary on your computer or a GitHub runner, uploads it over
SSH, restarts the service, and checks `GET /healthz`. When provider keys are
supplied, it also writes the server's credential file. If the new binary does
not become healthy, the script restores the previous binary and credentials.

## One-time server setup

Use the account that runs Meal Planner, normally `ubuntu`. It needs a working
`systemctl --user` session and lingering enabled so its services start at boot.
Meal Planner's bot setup may have already done this:

```bash
sudo loginctl enable-linger ubuntu
```

The deployment workflow reads `GROQ_API_KEY` and `MISTRAL_API_KEY` from GitHub
secrets and creates `~/.config/model-fleet/router.env` on the server with mode
`600` in a directory with mode `700`. Both keys are required at startup because
the embedded router configuration defines both provider connections. This file
is separate from Meal Planner's `.env`.

## Deploy from the command line

From this repository, use a direct SSH target or an SSH config alias:

```bash
./scripts/deploy.sh ubuntu@YOUR_HOST
./scripts/deploy.sh ubuntu@YOUR_HOST /path/to/private-key
```

With both `GROQ_API_KEY` and `MISTRAL_API_KEY` set in your local environment,
the script creates or updates the server's `router.env` without printing the
keys. When neither variable is set, it uses the existing server file and fails
if that file does not exist. Supplying only one key is an error. Load the keys
from your secret manager before running the first command-line deployment.

The script accepts Linux `amd64` and `arm64` hosts. It installs
`~/.local/bin/model-fleet-router` and
`~/.config/systemd/user/model-fleet-router.service`, enables the service, and
checks its local health endpoint. Useful remote checks:

```bash
ssh ubuntu@YOUR_HOST 'systemctl --user status model-fleet-router.service'
ssh ubuntu@YOUR_HOST 'journalctl --user -u model-fleet-router.service -n 100 --no-pager'
ssh ubuntu@YOUR_HOST 'curl --fail http://127.0.0.1:4545/healthz'
```

For a local Analyst eval against the deployed router, open an SSH tunnel in
another terminal:

```bash
ssh -N -L 4545:127.0.0.1:4545 ubuntu@YOUR_HOST
```

The current Meal Planner eval uses `http://localhost:4545`, so the tunnel makes
that URL reach the remote service. Its GitHub Actions runner will also need a
tunnel or a locally started router before that eval can run in CI.

## Automatic deployment from GitHub Actions

The [CI workflow](../.github/workflows/ci.yml) runs the race-enabled Go suite
on pushes and pull requests. A successful push to `main` then deploys to the
server. Set the deployment secrets in this repository's `production`
environment. The provider keys can be repository secrets or `production`
environment secrets:

| Secret | Value |
| --- | --- |
| `DEPLOY_TARGET` | SSH target, such as `ubuntu@YOUR_HOST` |
| `DEPLOY_KEY` | Private SSH key authorized for that account |
| `DEPLOY_KNOWN_HOSTS` | Verified SSH host-key line for `YOUR_HOST` |
| `GROQ_API_KEY` | Groq provider key written to `router.env` |
| `MISTRAL_API_KEY` | Mistral provider key written to `router.env` |

You can obtain a host-key line with `ssh-keyscan`, but verify its fingerprint
against the server's own SSH host key before storing it. The workflow requires
host-key checking. The API keys may already exist as repository secrets for the
provider acceptance workflow; that workflow cannot read secrets scoped only to
the `production` environment. The deploy job can use repository secrets or
`production` environment secrets. Set all five secrets before merging the
deployment workflow into `main`.
