# faultbrief

[한국어 문서 (Korean)](README.ko.md)

Turn Linux logs into an incident brief. One command, no daemon, no setup.

```
faultbrief --since 1h
```

faultbrief reads journald, syslog, and nginx logs, and detects incident
signals with a rule engine:

- **Signatures** — 21 curated known-bad patterns (OOM kills, disk full,
  systemd unit failures, nginx upstream errors, ...)
- **Spikes** — frequency anomalies vs. the preceding baseline window
  (5-minute buckets, z-score with minimum-count guards)
- **New patterns** — log templates never seen in the baseline
  (variable-masking fingerprints)

An LLM is **optional**: `--llm` adds an AI incident briefing on top of the
rule-engine report (Anthropic API or any OpenAI-compatible endpoint,
including local Ollama). Without it, faultbrief is still a complete tool.

## Install

### From source

```bash
go install github.com/LeeSeokBln/faultbrief/cmd/faultbrief@latest
```

Or alias it for convenience:

```bash
alias brief=faultbrief
```

### From release tarball

Download the latest release from
[GitHub Releases](https://github.com/LeeSeokBln/faultbrief/releases),
extract, and add to your PATH:

```bash
tar xzf faultbrief_*_linux_amd64.tar.gz
sudo mv faultbrief /usr/local/bin/
```

Supported platforms: Linux (amd64, arm64), macOS (amd64, arm64).

## Usage

```
faultbrief [flags]

Flags:
  --since DURATION        analysis window size (e.g. 30m, 1h, 1d; default "1h")
  --until DURATION        skip the most recent span (e.g. 10m; default "0")
  --baseline DURATION     baseline span before the window (default "24h")
  --format FORMAT         output format: text, md, json (default "text")
  --lang LANG             report language: en, ko (default "en")
  --min-severity SEV      minimum finding severity to report:
                          debug, info, notice, warning, error, critical
                          (default "info")
  --llm                   add an LLM incident briefing
  --source NAMES          limit sources: journald, syslog, nginx,
                          nginx-access, nginx-error (default: all detected)
  --syslog-path PATHS         syslog file paths/globs (default: /var/log/syslog*, /var/log/messages*)
  --nginx-access-path PATHS   nginx access log paths/globs (default: /var/log/nginx/access.log*)
  --nginx-error-path PATHS    nginx error log paths/globs (default: /var/log/nginx/error.log*)
  --rules FILES           extra signature rule YAML files
  --use-cache             remember templates across runs, so "new pattern"
                          means new-ever, not just new-vs-yesterday
  --no-color              disable colored output
  --config PATH           config file (default ~/.config/faultbrief/config.yaml)

Commands:
  version                 print version

Exit codes (cron/CI friendly):
  0  no findings — logs look healthy
  1  findings detected
  2  runtime error (bad flags, no readable sources, ...)
```

Reading `/var/log` usually requires membership in the `adm` group or sudo.

## Example output

Real output (from the test fixtures):

```
FAULTBRIEF — incident brief
Window: 2026-07-07 09:00 → 2026-07-07 10:00 (baseline: previous 24h0m0s)
Sources: journald ✓ (1) · syslog ✓ (69) · nginx-access ✓ (260) · nginx-error ✓ (3)
Records: 104 · Findings: 8

[CRITICAL] signature oom-kill — OOM killer terminated a process (×2)
  first: 09:12:00 · last: 09:14:00 · unit: kernel · syslog
  matched 2 time(s)
  > Out of memory: Killed process 1234 (myapp) total-vm:204800kB
  hint: free -h; dmesg | grep -i oom; check the unit's memory limits

[ERROR] spike nginx-5xx-rate — nginx 5xx error rate is high (×12)
  first: 09:00:00 · last: 10:00:00 · unit: nginx · nginx-access
  12 of 60 requests failed (20.0%), baseline 0.0%

[ERROR] spike 2fa0ade5fedf824e — error: pg query timeout after <NUM> ms (×30)
  first: 09:00:00 · last: 09:58:00 · unit: myapp · syslog
  30 occurrences (2.5/bucket) vs baseline 0.02/bucket — z=5.0, ×50.0
  > error: pg query timeout after 5000 ms
```

Every finding says *why* it was raised: the rule that matched, or the exact
counts and z-score behind a spike.

## LLM briefing (optional)

Enable an AI incident summary with `--llm`. The rule-engine report always
prints in full — if the LLM call fails, you lose nothing but the summary.

### Anthropic API

```bash
export ANTHROPIC_API_KEY=sk-ant-...
faultbrief --since 1h --llm
```

### OpenAI API

```bash
export OPENAI_API_KEY=sk-...
export FAULTBRIEF_LLM_PROVIDER=openai
export FAULTBRIEF_LLM_MODEL=gpt-4o-mini
faultbrief --since 1h --llm
```

### Local Ollama (no cloud)

```bash
export FAULTBRIEF_LLM_PROVIDER=openai
export FAULTBRIEF_LLM_BASE_URL=http://localhost:11434
export FAULTBRIEF_LLM_MODEL=llama3
faultbrief --since 1h --llm
```

Settings can also live in `~/.config/faultbrief/config.yaml`:

```yaml
lang: ko
llm:
  provider: openai
  base_url: http://localhost:11434
  model: llama3
```

Precedence: flags > environment > config file > defaults.

## Custom rules

Add your own signatures in a YAML file:

```yaml
- id: my-app-crash
  title: "My app crashed"
  severity: critical
  source: syslog            # optional: journald, syslog, nginx-access, nginx-error, any
  contains: "my-app: segmentation fault"
  hint: "coredumpctl list; check the last deploy"
  example: "my-app: segmentation fault at 0x0"

- id: slow-query
  title: "Database query over limit"
  severity: error
  regex: "query took \\d+ms, exceeded"
  hint: "EXPLAIN ANALYZE the query; check missing indexes"
  example: "db: query took 5000ms, exceeded limit"
```

Load with `--rules ./my-rules.yaml`. Exactly one of `contains`/`regex` per
rule. Builtin rules are always loaded.

## 한국어

한국어 리포트와 LLM 브리핑을 지원합니다. 자세한 내용은
[README.ko.md](README.ko.md)를 보세요.

```bash
faultbrief --since 1h --lang ko
```

## License

MIT — see [LICENSE](LICENSE).
