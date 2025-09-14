# Reservation Worker

Traffic Tacos 프로젝트의 비동기 이벤트 처리 워커 서비스입니다. 예약 만료 및 결제 결과를 처리하여 재고 관리 및 예약 상태 관리를 수행합니다.

## 📋 목차

- [기능](#기능)
- [아키텍처](#아키텍처)
- [이벤트 타입](#이벤트-타입)
- [설치 및 실행](#설치-및-실행)
- [환경변수 설정](#환경변수-설정)
- [개발](#개발)
- [테스트](#테스트)
- [모니터링](#모니터링)
- [배포](#배포)

## 🎯 기능

- **SQS 이벤트 소비**: EventBridge/SQS에서 이벤트 메시지를 소비
- **비동기 워크플로우 처리**: 예약 만료, 결제 승인/실패 이벤트 처리
- **멱등성 보장**: 동일 이벤트의 중복 처리를 방지
- **재시도 로직**: 외부 서비스 호출 실패 시 지수 백오프 재시도
- **관측성**: OpenTelemetry 트레이싱, Prometheus 메트릭, 구조화된 로깅
- **그레이스풀 셧다운**: 시그널 기반 안전한 서비스 중단
- **오토스케일링 지원**: KEDA 기반 SQS 큐 길이에 따른 Pod 수 자동 조정

## 🏗️ 아키텍처

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  EventBridge│    │     SQS     │    │Reservation │
│ /Payment API │───▶│   Queue     │───▶│  Worker    │
└─────────────┘    └─────────────┘    └─────┬──────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
           ┌────────▼────────┐    ┌────────▼────────┐    ┌────────▼────────┐
           │Inventory Service│    │Reservation API │    │  Observability  │
           │    (gRPC)       │    │    (REST)      │    │  (OTel/Prom/Zap) │
           └─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 컴포넌트 설명

- **Worker Pool**: 설정된 동시성만큼 고루틴을 생성하여 이벤트 처리
- **SQS Poller**: Long polling으로 메시지를 효율적으로 소비
- **Event Handler**: 이벤트 타입별 비즈니스 로직 실행
- **Clients**: Inventory(gRPC), Reservation(REST) 서비스와 통신
- **Observability**: 분산 추적, 메트릭 수집, 구조화된 로깅

## 📨 이벤트 타입

### 1. reservation.expired
예약 홀드 시간이 만료되어 재고를 다시 릴리즈해야 하는 경우

```json
{
  "id": "evt_123456",
  "type": "reservation.expired",
  "reservation_id": "rsv_abc123",
  "event_id": "evt_2025_1001",
  "ts": "2024-01-01T12:00:00Z",
  "payload": {
    "qty": 2,
    "seat_ids": ["A-12", "A-13"]
  }
}
```

**처리 로직**:
1. Reservation API: 예약 상태를 `EXPIRED`로 업데이트
2. Inventory Service: 재고 홀드를 해제

### 2. payment.approved
결제가 승인되어 예약을 확정해야 하는 경우

```json
{
  "id": "evt_123457",
  "type": "payment.approved",
  "reservation_id": "rsv_abc123",
  "event_id": "evt_2025_1001",
  "ts": "2024-01-01T12:05:00Z",
  "payload": {
    "payment_intent_id": "pay_xyz789",
    "amount": 120000
  }
}
```

**처리 로직**:
1. Reservation API: 예약 상태를 `CONFIRMED`로 업데이트

### 3. payment.failed
결제가 실패하여 예약을 취소해야 하는 경우

```json
{
  "id": "evt_123458",
  "type": "payment.failed",
  "reservation_id": "rsv_abc123",
  "event_id": "evt_2025_1001",
  "ts": "2024-01-01T12:10:00Z",
  "payload": {
    "payment_intent_id": "pay_xyz789",
    "amount": 120000
  }
}
```

**처리 로직**:
1. Reservation API: 예약 상태를 `CANCELLED`로 업데이트

## 🚀 설치 및 실행

### 사전 요구사항

- Go 1.22+
- AWS CLI (로컬 개발용)
- Docker (로컬 개발용)

### 로컬 실행

1. **의존성 설치**:
```bash
make deps
```

2. **환경변수 설정**:
```bash
export SQS_QUEUE_URL="https://sqs.ap-northeast-2.amazonaws.com/123/queue"
export INVENTORY_GRPC_ADDR="inventory-svc:8080"
export RESERVATION_API_BASE="http://reservation-api:8080"
# ... 다른 환경변수들
```

3. **빌드 및 실행**:
```bash
make build
./reservation-worker
```

### Docker 실행

```bash
# 빌드
make docker-build

# 실행
docker run -e SQS_QUEUE_URL="..." reservation-worker:latest
```

## ⚙️ 환경변수 설정

| 변수 | 필수 | 기본값 | 설명 |
|------|------|--------|------|
| `SQS_QUEUE_URL` | ✅ | - | SQS 큐 URL |
| `SQS_WAIT_TIME` | ❌ | 20 | SQS long polling 대기 시간 (초) |
| `WORKER_CONCURRENCY` | ❌ | 20 | 워커 풀 고루틴 수 |
| `MAX_RETRIES` | ❌ | 5 | 최대 재시도 횟수 |
| `BACKOFF_BASE_MS` | ❌ | 1000 | 재시도 백오프 기본 시간 (ms) |
| `INVENTORY_GRPC_ADDR` | ✅ | - | Inventory 서비스 gRPC 주소 |
| `RESERVATION_API_BASE` | ✅ | - | Reservation API 기본 URL |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | ❌ | http://otel-collector:4317 | OpenTelemetry 엔드포인트 |
| `LOG_LEVEL` | ❌ | info | 로그 레벨 (debug/info/warn/error) |

### 환경별 설정 예시

**개발환경**:
```bash
export LOG_LEVEL=debug
export WORKER_CONCURRENCY=5
export MAX_RETRIES=3
```

**운영환경**:
```bash
export LOG_LEVEL=info
export WORKER_CONCURRENCY=50
export SQS_WAIT_TIME=20
```

## 💻 개발

### 프로젝트 구조

```
.
├── cmd/reservation-worker/     # 메인 애플리케이션
├── internal/
│   ├── client/                 # 외부 서비스 클라이언트
│   ├── config/                 # 설정 관리
│   ├── handler/                # 이벤트 핸들러
│   ├── observability/          # 로깅/메트릭/트레이싱
│   └── worker/                 # SQS 폴링 및 워커 관리
├── pkg/types/                  # 공통 타입 정의
├── test/                       # 테스트
│   ├── integration/            # 통합 테스트
│   └── unit/                   # 단위 테스트
├── Dockerfile                  # 컨테이너 이미지
├── Makefile                    # 빌드/테스트 스크립트
└── README.md                   # 이 파일
```

### 개발 명령어

```bash
# 코드 포맷팅
make fmt

# 코드 검증
make vet

# 린팅
make lint

# 테스트 실행
make test

# 테스트 커버리지
make test-coverage

# 개발용 빌드 (hot reload 포함)
make dev-air  # air가 설치된 경우
```

### 코드 추가/수정 가이드

1. **새 이벤트 타입 추가**:
   - `pkg/types/event.go`에 타입과 페이로드 구조체 추가
   - `internal/handler/handler.go`에 처리 로직 구현
   - 테스트 케이스 추가

2. **새 메트릭 추가**:
   - `internal/observability/metrics.go`에 메트릭 정의
   - 핸들러에서 메트릭 기록

3. **새 클라이언트 추가**:
   - `internal/client/`에 새 클라이언트 구현
   - `internal/config/config.go`에 설정 추가

## 🧪 테스트

### 단위 테스트

```bash
# 모든 단위 테스트 실행
make test

# 특정 패키지 테스트
go test ./internal/config/...

# 자세한 출력
go test -v ./internal/config/
```

### 통합 테스트

LocalStack을 사용한 SQS 통합 테스트:

```bash
# LocalStack 시작
docker run -d -p 4566:4566 localstack/localstack

# 통합 테스트 실행
go test -tags=integration ./test/integration/
```

### 테스트 커버리지

```bash
make test-coverage
# coverage.html 파일이 생성됩니다
```

## 📊 모니터링

### 메트릭

Prometheus 메트릭 엔드포인트에서 다음 메트릭을 제공:

- `worker_events_total{type, outcome}`: 이벤트 처리 수
- `worker_latency_seconds_bucket{type}`: 이벤트 처리 지연 시간
- `sqs_poll_errors_total`: SQS 폴링 에러 수
- `worker_pool_active_gauge`: 활성 워커 수

### 로그

구조화된 JSON 로그 출력:

```json
{
  "ts": "2024-01-01T12:00:00Z",
  "level": "info",
  "event_type": "reservation.expired",
  "reservation_id": "rsv_abc123",
  "attempt": 1,
  "latency_ms": 150,
  "trace_id": "abc123...",
  "msg": "Event processed successfully"
}
```

### 트레이싱

OpenTelemetry를 통한 분산 추적 지원:
- 각 이벤트 처리에 대한 트레이스 생성
- 외부 서비스 호출에 대한 스팬 생성
- Jaeger/Tempo 등과 연동 가능

## 🚢 배포

### Kubernetes 배포

KEDA를 사용한 오토스케일링 설정:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: reservation-worker-scaler
spec:
  scaleTargetRef:
    name: reservation-worker
  triggers:
  - type: aws-sqs-queue
    metadata:
      queueURL: "https://sqs.ap-northeast-2.amazonaws.com/123/queue"
      queueLength: "10"  # 큐 길이가 10개 이상이면 스케일링 시작
      awsRegion: "ap-northeast-2"
  minReplicaCount: 0
  maxReplicaCount: 50
```

### Helm 차트

```bash
# Helm 차트 배포
helm upgrade --install reservation-worker ./helm/reservation-worker \
  --set image.tag=v1.0.0 \
  --set config.sqsQueueUrl="https://sqs.ap-northeast-2.amazonaws.com/123/queue"
```

### Docker 이미지

```bash
# Multi-arch 이미지 빌드
docker buildx build --platform linux/amd64,linux/arm64 -t reservation-worker:latest .
```

## 🤝 기여

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### 커밋 메시지 규칙

```
feat: 새로운 기능 추가
fix: 버그 수정
docs: 문서 수정
style: 코드 스타일 수정 (포맷팅 등)
refactor: 코드 리팩토링
test: 테스트 추가/수정
chore: 빌드/설정 파일 변경
```

## 📝 라이선스

이 프로젝트는 MIT 라이선스 하에 있습니다.

## 📞 지원

질문이나 이슈가 있으시면 [GitHub Issues](https://github.com/traffic-tacos/reservation-worker/issues)를 이용해주세요.

