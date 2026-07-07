# Deploy CodePilot AI for free — Oracle Cloud + Docker + Cloudflare Tunnel

This runs the **full stack** (backend + Postgres + frontend + the on-demand GitHub MCP container)
on a free Oracle Cloud "Always Free" VM, exposed over HTTPS with a free Cloudflare Tunnel — so real
PR reviews work end-to-end. Everything below is copy-paste.

> **Why a VM (not Render/Vercel for the backend):** the backend spawns the GitHub MCP server as a
> Docker container, which needs a real Docker host. A VM gives you that; free PaaS does not.

---

## 0. What you need
- An **Oracle Cloud** account (free; card for identity check, not charged).
- A **GitHub Personal Access Token** with `repo` scope, and a **webhook secret** (any random string).
- An **LLM API key** (e.g. your Groq key).
- *(For a stable public URL)* optionally a domain on **Cloudflare** (free plan). Without one, you can
  still use an instant free `*.trycloudflare.com` URL (changes if the tunnel restarts).

---

## 1. Create the free VM (step by step)

**1.1 — Make an Oracle Cloud account.** Go to <https://www.oracle.com/cloud/free/> → **Start for
free**. Verify email + phone, add a card (identity check only — Always Free resources are never
charged). Pick a **home region** close to you (ARM capacity varies by region; a less-popular region
is easier to get).

**1.2 — Create an SSH key on your laptop** (skip if you already have one):
```bash
ssh-keygen -t ed25519 -f ~/.ssh/oracle_codepilot -C "codepilot"
# creates ~/.ssh/oracle_codepilot (private) and ~/.ssh/oracle_codepilot.pub (public)
```

**1.3 — Launch the instance.** In the OCI Console: ☰ menu → **Compute → Instances → Create instance**.
- **Name:** `codepilot`
- **Image:** click **Edit** → **Change image** → **Canonical Ubuntu** → 22.04.
- **Shape:** click **Change shape** → **Ampere** → **VM.Standard.A1.Flex** → set **2 OCPUs** and
  **12 GB** (well within the Always-Free 4 OCPU / 24 GB).
  - If you see **"Out of host capacity"**: lower to 1 OCPU / 6 GB, pick a different **Availability
    Domain**, or try again later / another region. (This is the one common friction point on the free
    ARM tier — just retry.)
- **Networking:** leave defaults (it creates a VCN + public subnet). Ensure **Assign a public IPv4
  address = Yes**.
- **Add SSH keys:** choose **Paste public keys** and paste the contents of
  `~/.ssh/oracle_codepilot.pub`.
- Click **Create**. Wait ~1 minute until state is **Running**, then copy the **Public IP address**.

> You do **not** need to open any ingress ports — the Cloudflare Tunnel connects **outbound**, so the
> VM stays fully closed to the internet.

**1.4 — SSH in:**
```bash
ssh -i ~/.ssh/oracle_codepilot ubuntu@<PUBLIC_IP>
```

## 2. Install Docker
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker   # run docker without sudo
docker --version && docker compose version
```

## 3. Get the code + configure secrets
```bash
git clone https://github.com/harshittaneja13/CodePilot-AI.git
cd CodePilot-AI
cp .env.example .env
nano .env
```
Set at minimum:
```dotenv
APP_ENVIRONMENT=production           # requires the secrets below to be set

# GitHub
GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxx
GITHUB_WEBHOOK_SECRET=<a-long-random-string>

# LLM (Groq example — OpenAI-compatible)
LLM_PROVIDER=openai
LLM_BASE_URL=https://api.groq.com/openai/v1
LLM_API_KEY=gsk_xxx
LLM_MODEL=llama-3.3-70b-versatile

# Database — use a strong password for a public deploy
DB_PASSWORD=<a-strong-password>

# Keep RAG off on a small VM (it adds Qdrant + Ollama, which need RAM)
RAG_ENABLED=false
```
*(You can set `WEBHOOK_BASE_URL` to your public URL later so repos auto-register their webhook.)*

## 4. Start the stack
Use the production compose file — it runs only the 4 needed services (no RAM-heavy Qdrant/Ollama) and
binds ports to loopback only (nothing exposed on the public IP):
```bash
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f backend   # startup + migrations (Ctrl-C to stop)
```
Local check on the VM:
```bash
curl -s http://localhost:8080/api/health      # → {"status":"ok"}
curl -s http://localhost:3000 | head          # frontend HTML
```
`frontend` (nginx, port 3000) serves the dashboard **and** proxies `/api` to the backend — so a single
public URL exposes everything.

## 5. Public HTTPS URL — Cloudflare Tunnel (free)
Install cloudflared (ARM64):
```bash
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64 \
  -o cloudflared && chmod +x cloudflared && sudo mv cloudflared /usr/local/bin/
```

**Option A — instant, zero-config (ephemeral URL):**
```bash
cloudflared tunnel --url http://localhost:3000
```
It prints `https://<random>.trycloudflare.com` — your public app URL. It runs in the foreground; to
keep it alive across logouts/reboots, install the provided systemd unit:
```bash
sudo cp deploy/cloudflared-quick.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now cloudflared-quick
# print the current public URL:
journalctl -u cloudflared-quick -n 40 --no-pager | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com'
```
> The quick-tunnel URL **changes whenever cloudflared restarts**. Fine for demoing / screen-recording;
> for a permanent URL use Option B.

**Option B — stable URL (needs a domain on Cloudflare, free plan):**
```bash
cloudflared tunnel login                       # authorize your Cloudflare domain
cloudflared tunnel create codepilot
cloudflared tunnel route dns codepilot app.yourdomain.com
# ~/.cloudflared/config.yml:
#   tunnel: codepilot
#   credentials-file: /home/ubuntu/.cloudflared/<id>.json
#   ingress:
#     - hostname: app.yourdomain.com
#       service: http://localhost:3000
#     - service: http_status:404
sudo cloudflared service install               # run on boot
```
Now `https://app.yourdomain.com` is your permanent URL.

Open the URL in a browser → the CodePilot dashboard loads, talking to your live backend.

## 6. (Optional) Wire GitHub webhooks for auto-reviews
For a repo you want auto-reviewed: GitHub repo → **Settings → Webhooks → Add webhook**:
- **Payload URL:** `https://<your-public-url>/api/webhooks/github`
- **Content type:** `application/json`
- **Secret:** the same `GITHUB_WEBHOOK_SECRET`
- **Events:** "Let me select" → **Pull requests**

Or just use the dashboard: add the repo, open a PR, hit **Run Review**.

---

## Maintenance
```bash
cd ~/CodePilot-AI
git pull
docker compose -f docker-compose.prod.yml up -d --build   # redeploy
docker compose -f docker-compose.prod.yml logs -f backend # logs
docker compose -f docker-compose.prod.yml down            # stop
```

## Troubleshooting
- **`exec format error` / MCP container won't start:** an architecture mismatch. The Dockerfile now
  builds for the host arch (ARM/x86) automatically; make sure you built **on the VM** (`--build`).
- **Backend exits on start:** in `production` it requires `GITHUB_PERSONAL_ACCESS_TOKEN`,
  `GITHUB_WEBHOOK_SECRET`, and `LLM_API_KEY`. Check `docker compose logs backend`.
- **Out of memory:** you left Qdrant/Ollama running — start only the four services in step 4, or pick
  a larger ARM shape (still free up to 24 GB).
- **Webhook 401:** the secret in GitHub must match `GITHUB_WEBHOOK_SECRET`.
- **Want direct IP access (no tunnel):** open port 3000 in **both** the OCI Security List *and* the
  Ubuntu firewall (`sudo iptables -I INPUT -p tcp --dport 3000 -j ACCEPT`) — but the tunnel is easier
  and gives HTTPS.

## Cost
Stays within Oracle **Always Free** (VM) + Cloudflare **free** (tunnel) + your free LLM tier — **$0/mo**.
A custom domain (Option B) is the only optional paid piece (~a few dollars/year).

## Tip for your resume
Keep the permanent **Vercel demo** as your resume link (always-on, zero-maintenance). Use this VM to
demo/screen-record the *real* thing reviewing PRs, or make it permanent with Option B (domain).
