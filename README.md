# faultbrief

Turn Linux logs into an incident brief. One command, no daemon, no setup.

```
faultbrief --since 1h
```

faultbrief reads journald, syslog, and nginx logs, and detects incident
signals with a rule engine:

- **Signatures** — 21 curated known-bad patterns (OOM kills, disk full,
  systemd unit failures, nginx upstream errors, ...)
- **Spikes** — error-rate anomalies vs. the preceding baseline window
  (5-minute buckets, z-score with minimum-count guards)
- **New patterns** — log templates never seen in the baseline
  (variable-masking fingerprints)

An LLM is **optional**: `--llm` adds an AI incident briefing on top of the
rule-engine report (Anthropic API or any OpenAI-compatible endpoint,
including local Ollama). Without it, faultbrief is still a complete tool.

## Install

### macOS / Linux (from source)

```bash
go install github.com/LeeSeokBln/faultbrief/cmd/faultbrief@latest
```

Or alias it for convenience:
```bash
alias brief=faultbrief
```

### From release tarball

Download the latest release from [GitHub Releases](https://github.com/LeeSeokBln/faultbrief/releases), extract, and add to your PATH:

```bash
tar xzf faultbrief_linux_amd64.tar.gz
sudo mv faultbrief /usr/local/bin/
```

Supported platforms: Linux (amd64, arm64), macOS (amd64, arm64).

## Usage

```
faultbrief [flags]

Flags:
  --since DURATION       Look back this duration (e.g. 1h, 30m; default "1h")
  --until DURATION       Look forward from now (default "0s" = now)
  --format FORMAT        Output format: text, json, markdown (default "text")
  --lang LANG            Output language: en, ko (default "en")
  --min-severity SEV     Only show findings at or above severity:
                         debug, info, notice, warning, error, critical
                         (default "notice")
  --llm                  Add AI-generated incident briefing (requires ANTHROPIC_API_KEY
                         or FAULTBRIEF_LLM_* environment variables)
  --source SOURCE        Only read logs from specified source (syslog, journald,
                         nginx-access, nginx-error; default: all available)
  --use-cache            Cache template fingerprints to speed up repeated runs
  --rules FILE           Load custom rules from YAML file (in addition to builtins)
  --help                 Show help message
  --version              Print version

Exit codes:
  0  No significant findings (below --min-severity threshold)
  1  Findings detected
  2  Error (invalid flags, I/O error, etc.)
```

## Example output

```
=== INCIDENT FINDINGS (since 1h ago) ===

CRITICAL: OOM killer terminated a process
  Rule: oom-kill
  Found 1 occurrence (1 since baseline)
  Source: journald (kernel)
  Sample:
    kernel: Out of memory: Killed process 1234 (myapp) total-vm:2048kB
  Hint: free -h; dmesg | grep -i oom; check the unit's memory limits

ERROR: nginx has no live upstreams
  Rule: nginx-no-live-upstreams
  Found 3 occurrences (3 new patterns since baseline)
  Source: nginx (nginx-error)
  Details: All backends down — check the upstream service and health checks

NOTICE: Spike in error-rate anomaly (5xx class)
  Detector: spike
  Baseline: 2 errors per 5min, Current window: 15 errors per 5min
  Score: z=2.8 (>2.0 threshold)
  Duration: 15 minutes
```

## LLM briefing (optional)

Enable AI-powered incident analysis by setting API credentials and adding `--llm` to the command:

### Using Anthropic API

```bash
export ANTHROPIC_API_KEY=sk-ant-...
faultbrief --since 1h --llm
```

### Using OpenAI API

```bash
export OPENAI_API_KEY=sk-...
export FAULTBRIEF_LLM_PROVIDER=openai
faultbrief --since 1h --llm
```

### Using local Ollama

```bash
# Start Ollama server
ollama serve

# In another terminal, run faultbrief
export FAULTBRIEF_LLM_PROVIDER=openai
export FAULTBRIEF_LLM_BASE_URL=http://localhost:11434/v1
export FAULTBRIEF_LLM_MODEL=llama2
faultbrief --since 1h --llm
```

## Custom rules

Create a YAML file with custom signature rules:

```yaml
- id: my-app-crash
  title: "My app crashed"
  severity: critical
  source: syslog
  contains: "my-app crashed: segmentation fault"
  hint: "Check logs in /var/log/my-app.log; restart the service"
  example: "my-app crashed: segmentation fault at address 0x12345"

- id: slow-query
  title: "Database query timeout"
  severity: error
  source: syslog
  regex: "Query took (\\d+)ms, exceeded (\\d+)ms limit"
  hint: "Run ANALYZE on the table; add an index if needed"
  example: "Query took 5000ms, exceeded 3000ms limit"
```

Load with `--rules ./custom-rules.yaml`. Builtin rules are always loaded.

## 한국어

faultbrief는 한국어를 지원합니다.

```bash
faultbrief --since 1h --lang ko
```

LLM 브리핑도 한국어로 출력됩니다:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
faultbrief --since 1h --lang ko --llm
```

출력 예시:

```
=== 인시던트 발견 (지난 1시간) ===

CRITICAL: OOM killer가 프로세스를 종료했습니다
  규칙: oom-kill
  발생 횟수: 1회 (기준 이후 1회)
  출처: journald (kernel)
  샘플:
    kernel: Out of memory: Killed process 1234 (myapp) total-vm:2048kB
  힌트: free -h; dmesg | grep -i oom; unit의 메모리 제한을 확인하세요
```

## License

MIT - See LICENSE file for details.
