# Claude Monitor

Claude Code CLI의 사용량(토큰)을 주기적으로 수집하여 서버에 업로드하는 백그라운드 모니터링 도구입니다.

## 기능

- `~/.claude/projects/` 디렉토리의 JSONL 파일에서 사용량 데이터 수집
- 메시지 ID 기반 중복 제거
- 일별 토큰 사용량 집계 (최근 90일)
- 주기적으로 서버에 업로드 (기본 10분)
- macOS/Windows 로그인 시 자동 시작 지원

## 설치

### 빌드된 바이너리 사용

바이너리 파일을 전달받은 후:

```bash
# 실행 권한 부여
chmod +x claude-monitor

# Gatekeeper 우회 (다음 중 하나 선택)

# 방법 1: quarantine 속성 제거
xattr -d com.apple.quarantine claude-monitor

# 방법 2: ad-hoc 코드사인
codesign --sign - --force claude-monitor
```

### 소스에서 빌드

```bash
# 저장소 클론
git clone https://github.com/rody-vx/claude-monitor.git
cd claude-monitor

# 빌드 (Go 1.21 이상 필요)
go build -o claude-monitor .

# 또는 모든 플랫폼용 빌드
bash build.sh
```

## 사용법

### 설치 (자동 시작 등록)

```bash
# 대화형 설정
./claude-monitor install

# 또는 CLI 인자로 설정
./claude-monitor install --email your@email.com
./claude-monitor install --email your@email.com --interval 300  # 5분 간격
./claude-monitor install --email your@email.com --server http://custom-server:3498
```

### 상태 확인

```bash
./claude-monitor status
```

### 제거

```bash
./claude-monitor uninstall
```

### 수동 실행 (포그라운드)

```bash
./claude-monitor run
```

### 테스트 (업로드 없이 데이터 수집만)

```bash
./claude-monitor test
```

## 설정

설정 파일: `~/.claude-monitor/config.json`

```json
{
  "email": "your@email.com",
  "serverUrl": "http://10.12.200.99:3498",
  "intervalSeconds": 600
}
```

| 설정 | 기본값 | 설명 |
|------|--------|------|
| `email` | (필수) | 사용자 이메일 |
| `serverUrl` | `http://10.12.200.99:3498` | 업로드 서버 URL |
| `intervalSeconds` | `600` (10분) | 업로드 주기 (초) |

## 파일 위치

| 파일 | 경로 |
|------|------|
| 설정 파일 | `~/.claude-monitor/config.json` |
| 로그 파일 | `~/.claude-monitor/monitor.log` |
| LaunchAgent (macOS) | `~/Library/LaunchAgents/com.claude.monitor.plist` |

## 자동 시작

### macOS

`install` 명령 실행 시 LaunchAgent가 등록되어 로그인할 때마다 자동으로 시작됩니다.

```bash
# 수동으로 서비스 제어
launchctl list | grep claude
launchctl stop com.claude.monitor
launchctl start com.claude.monitor
```

### Windows

`install` 명령 실행 시 Task Scheduler에 등록되어 로그온할 때마다 자동으로 시작됩니다.

```powershell
# 수동으로 서비스 제어
schtasks /Query /TN ClaudeMonitor
schtasks /End /TN ClaudeMonitor
schtasks /Run /TN ClaudeMonitor
```

## 업로드 데이터 형식

서버로 전송되는 JSON 형식:

```json
{
  "daily": [
    {
      "date": "2024-12-09",
      "totalInputTokens": 94054,
      "totalOutputTokens": 18637,
      "totalCacheWriteTokens": 348438,
      "totalCacheReadTokens": 3539585,
      "totalTokens": 4000714,
      "requestCount": 197
    }
  ]
}
```

## 트러블슈팅

### macOS에서 "확인되지 않은 개발자" 경고

```bash
# 방법 1: quarantine 속성 제거
xattr -d com.apple.quarantine ./claude-monitor

# 방법 2: ad-hoc 코드사인
codesign --sign - --force ./claude-monitor
```

### 서비스가 실행되지 않음

```bash
# 상태 확인
./claude-monitor status

# 로그 확인
cat ~/.claude-monitor/monitor.log

# 수동 실행으로 테스트
./claude-monitor run
```

### 데이터가 업로드되지 않음

1. 서버 URL 확인: `cat ~/.claude-monitor/config.json`
2. 네트워크 연결 확인
3. Claude Code 사용 기록 존재 여부: `ls ~/.claude/projects/`
