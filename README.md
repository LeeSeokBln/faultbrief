# faultbrief

**English** · [한국어](#한국어)

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

리눅스 로그를 장애 브리핑으로. 명령어 하나, 데몬 없음, 사전 설정 없음.

```
faultbrief --since 1h
```

faultbrief는 journald·syslog·nginx 로그를 읽어 **규칙 엔진**으로 장애 신호를
탐지합니다:

- **시그니처** — 엄선된 21종의 알려진 장애 패턴 (OOM kill, 디스크 풀,
  systemd 유닛 실패, nginx 업스트림 에러 등)
- **스파이크** — 직전 베이스라인 구간 대비 빈도 이상
  (5분 버킷, z-score + 최소 건수 가드로 오탐 방지)
- **신규 패턴** — 베이스라인에 없던 로그 템플릿
  (숫자/IP/UUID/경로를 마스킹한 핑거프린트 비교)

LLM은 **옵션**입니다. `--llm`을 붙이면 규칙 엔진 리포트 위에 AI 장애 브리핑이
추가됩니다 (Anthropic API 또는 OpenAI 호환 엔드포인트 — 로컬 Ollama 포함).
LLM 없이도 faultbrief는 그 자체로 완결된 도구입니다.

### 설치

#### 소스에서

```bash
go install github.com/LeeSeokBln/faultbrief/cmd/faultbrief@latest
```

짧게 쓰고 싶다면:

```bash
alias brief=faultbrief
```

#### 릴리즈 타르볼

[GitHub Releases](https://github.com/LeeSeokBln/faultbrief/releases)에서
받아서 PATH에 추가:

```bash
tar xzf faultbrief_*_linux_amd64.tar.gz
sudo mv faultbrief /usr/local/bin/
```

지원 플랫폼: Linux (amd64, arm64), macOS (amd64, arm64).

### 사용법

```
faultbrief [플래그]

플래그:
  --since 기간            분석 구간 크기 (예: 30m, 1h, 1d; 기본 "1h")
  --until 기간            가장 최근 구간 제외 (예: 10m; 기본 "0")
  --baseline 기간         분석 구간 직전의 베이스라인 길이 (기본 "24h")
  --format 포맷           출력 포맷: text, md, json (기본 "text")
  --lang 언어             리포트 언어: en, ko (기본 "en")
  --min-severity 심각도   이 심각도 이상만 리포트:
                          debug, info, notice, warning, error, critical
                          (기본 "info")
  --llm                   LLM 장애 브리핑 추가
  --source 이름들         소스 제한: journald, syslog, nginx,
                          nginx-access, nginx-error (기본: 자동 감지 전부)
  --syslog-path 경로들        syslog 파일 경로/글롭 (기본 /var/log/syslog*, /var/log/messages*)
  --nginx-access-path 경로들  nginx access 로그 경로/글롭 (기본 /var/log/nginx/access.log*)
  --nginx-error-path 경로들   nginx error 로그 경로/글롭 (기본 /var/log/nginx/error.log*)
  --rules 파일들          사용자 시그니처 규칙 YAML 추가
  --use-cache             실행 간 템플릿 기억 — "신규 패턴"이 "어제 대비 신규"가
                          아니라 "역대 최초"를 뜻하게 됨
  --no-color              컬러 출력 끄기
  --config 경로           설정 파일 (기본 ~/.config/faultbrief/config.yaml)

명령:
  version                 버전 출력

종료 코드 (cron/CI 연동용):
  0  발견 없음 — 로그 정상
  1  발견 있음
  2  실행 에러 (잘못된 플래그, 읽을 수 있는 소스 없음 등)
```

`/var/log`를 읽으려면 보통 `adm` 그룹 소속이거나 sudo가 필요합니다.

### 출력 예시

테스트 픽스처 기준 실제 출력:

```
FAULTBRIEF — 장애 브리핑
분석 구간: 2026-07-07 09:00 → 2026-07-07 10:00 (베이스라인: 직전 24h0m0s)
소스: journald ✓ (1) · syslog ✓ (69) · nginx-access ✓ (260) · nginx-error ✓ (3)
레코드: 104 · 발견: 8

[심각] 시그니처 oom-kill — OOM killer terminated a process (×2)
  최초: 09:12:00 · 최종: 09:14:00 · 유닛: kernel · syslog
  matched 2 time(s)
  > Out of memory: Killed process 1234 (myapp) total-vm:204800kB
  힌트: free -h; dmesg | grep -i oom; check the unit's memory limits

[에러] 스파이크 nginx-5xx-rate — nginx 5xx error rate is high (×12)
  최초: 09:00:00 · 최종: 10:00:00 · 유닛: nginx · nginx-access
  12 of 60 requests failed (20.0%), baseline 0.0%

[에러] 스파이크 2fa0ade5fedf824e — error: pg query timeout after <NUM> ms (×30)
  최초: 09:00:00 · 최종: 09:58:00 · 유닛: myapp · syslog
  30 occurrences (2.5/bucket) vs baseline 0.02/bucket — z=5.0, ×50.0
  > error: pg query timeout after 5000 ms
```

모든 발견에는 **왜 잡혔는지**가 붙습니다: 매칭된 규칙, 또는 스파이크의
정확한 건수와 z-score.

### LLM 브리핑 (옵션)

`--llm`으로 AI 장애 요약을 켭니다. 규칙 엔진 리포트는 항상 전체 출력되며,
LLM 호출이 실패해도 요약만 빠질 뿐 아무것도 잃지 않습니다.

#### Anthropic API — 한국어 브리핑

```bash
export ANTHROPIC_API_KEY=sk-ant-...
faultbrief --since 1h --lang ko --llm
```

`--lang ko`면 AI 브리핑도 한국어로 나옵니다: 상황 요약, 원인 가설,
영향 범위, 다음 확인 커맨드 순.

#### 로컬 Ollama — 클라우드 없이

```bash
export FAULTBRIEF_LLM_PROVIDER=openai
export FAULTBRIEF_LLM_BASE_URL=http://localhost:11434
export FAULTBRIEF_LLM_MODEL=llama3
faultbrief --since 1h --lang ko --llm
```

설정 파일(`~/.config/faultbrief/config.yaml`)로도 가능:

```yaml
lang: ko
llm:
  provider: openai
  base_url: http://localhost:11434
  model: llama3
```

우선순위: 플래그 > 환경변수 > 설정 파일 > 기본값.

### 사용자 규칙

자기 서비스 전용 시그니처를 YAML로 추가:

```yaml
- id: my-app-crash
  title: "우리 앱 크래시"
  severity: critical
  source: syslog            # 생략 가능: journald, syslog, nginx-access, nginx-error, any
  contains: "my-app: segmentation fault"
  hint: "coredumpctl list; 마지막 배포 확인"
  example: "my-app: segmentation fault at 0x0"

- id: slow-query
  title: "DB 쿼리 시간 초과"
  severity: error
  regex: "query took \\d+ms, exceeded"
  hint: "EXPLAIN ANALYZE 실행; 인덱스 누락 확인"
  example: "db: query took 5000ms, exceeded limit"
```

`--rules ./my-rules.yaml`로 로드. 규칙마다 `contains`/`regex` 중 정확히
하나만. 내장 규칙은 항상 함께 로드됩니다.

### cron 연동 예시

매시간 검사해서 발견이 있으면 메일로:

```cron
0 * * * * faultbrief --since 1h --lang ko --format md > /tmp/brief.md || \
  [ $? -eq 1 ] && mail -s "faultbrief: 발견 있음" ops@example.com < /tmp/brief.md
```

---

## License

MIT — see [LICENSE](LICENSE).
