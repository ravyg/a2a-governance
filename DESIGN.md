# a2a-governance Design Document

## Overview

`a2a-governance` is a runtime governance middleware for A2A (Agent-to-Agent) protocol agents. It provides circuit breakers, policy evaluation, and human escalation hooks that plug into any `a2a-go` server with zero agent code changes.

The package addresses a critical gap in the autonomous agent ecosystem: **there is no standard way to govern agent behavior at runtime.** As AI agents autonomously execute transactions, make purchases, and interact with external services, organizations need runtime guardrails that can:

- Halt agent transactions when risk thresholds are exceeded
- Rate-limit agent activity to prevent runaway spending
- Detect anomalous behavior and escalate to humans
- Provide structured risk signals for payment networks and issuers

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Your Application                    │
│  (Ecommerce Platform, Payment Gateway, Agent App) │
├─────────────────────────────────────────────────┤
│              a2a-governance                       │
│  ┌────────────┐  ┌──────────┐  ┌────────────┐  │
│  │ interceptor │  │governance│  │   policy    │  │
│  │             │  │          │  │             │  │
│  │ CallIntrcpt │→ │ Circuit  │← │ ValueThresh │  │
│  │ RiskContext │  │ Breaker  │  │ Cumulative  │  │
│  └──────┬─────┘  └────┬─────┘  │ Velocity    │  │
│         │              │        │ Anomaly     │  │
│  ┌──────┴─────┐  ┌────┴─────┐  │ VendorTrust │  │
│  │   ap2      │  │  store   │  └─────────────┘  │
│  │ FCB Types  │  │ Memory   │                    │
│  │ RiskPaylod │  │ (Redis)  │                    │
│  │ Mandates   │  └──────────┘                    │
│  └────────────┘                                  │
├─────────────────────────────────────────────────┤
│              a2a-go SDK                          │
│  CallInterceptor · RequestContextInterceptor     │
│  AgentExecutor · RequestHandler                  │
├─────────────────────────────────────────────────┤
│              A2A Protocol                        │
│  JSON-RPC 2.0 · gRPC · Server-Sent Events       │
└─────────────────────────────────────────────────┘
```

## Package Structure

### `governance/` — Core primitives

The foundational package with no external dependencies beyond the standard library.

- **`BreakerState`** — Enum: CLOSED, OPEN, HALF_OPEN, TERMINATED
- **`CircuitBreaker`** — Thread-safe state machine with policy evaluation
- **`Policy`** — Interface for pluggable governance rules
- **`Evaluation`** — Result of a single policy evaluation
- **`EvaluationResult`** — Aggregated result across all policies
- **`EscalationHandler`** — Callback for human-in-the-loop intervention
- **`TransitionHook`** — Callback for state change observability

### `policy/` — Built-in policies

Ready-to-use policy implementations:

| Policy | Trip Condition | Use Case |
|--------|---------------|----------|
| `ValueThreshold` | Single transaction exceeds max | Prevent large unauthorized purchases |
| `CumulativeSpend` | Aggregate spend in window exceeds limit | Daily/weekly spend caps |
| `Velocity` | Request rate exceeds limit | Prevent rapid-fire transactions |
| `AnomalyDetection` | Value deviates from mean (Welford's) | Detect unusual agent behavior |
| `VendorTrust` | Vendor score below threshold | Block untrusted merchants |

### `interceptor/` — A2A SDK integration

Implements `a2asrv.CallInterceptor` and `a2asrv.RequestContextInterceptor`:

- **`GovernanceCallInterceptor`** — Evaluates policies before requests reach the agent executor. Blocks requests when the circuit is open.
- **`RiskContextInterceptor`** — Injects `RiskSignals` into the execution context so downstream agents can see the governance state.
- **`WithGovernance()`** / **`WithRiskContext()`** — Convenience `RequestHandlerOption` factories.

### `store/` — State persistence

- **`StateStore`** — Interface for persisting breaker state and audit events
- **`MemoryStore`** — In-memory implementation for testing and single-instance use
- Future: Redis, SQL, cloud-native implementations

### `ap2/` — AP2 protocol types

Types from AP2 Section 7.4 (Risk Signals):

- **`FCBEvaluation`** — Complete FCB evaluation with conditions and escalation
- **`TripConditionResult`** — Individual condition evaluation
- **`RiskPayload`** — Structured risk signal for mandate attachment
- **`IntentMandate`** / **`CartMandate`** / **`PaymentMandateContents`** — AP2 mandate types with `risk_payload` field

## Integration Points

### 1. A2A Server (via `a2a-go` SDK)

```go
handler := a2asrv.NewHandler(
    myExecutor,
    interceptor.WithGovernance(interceptor.GovernanceInterceptorConfig{
        Breaker:   breaker,
        Store:     auditStore,
        BreakerID: "production",
    }),
    interceptor.WithRiskContext(breaker),
)
```

The governance interceptor runs in the `CallInterceptor` chain **before** the request reaches `AgentExecutor.Execute()`. This means:

- No changes needed in agent code
- Works with any `AgentExecutor` implementation (ADK, custom, etc.)
- Can be added/removed via configuration

### 2. AP2 Mandates

When an agent prepares a payment mandate, the risk payload is attached:

```go
result, _ := breaker.Evaluate(ctx, reqCtx)
riskPayload := ap2.NewRiskPayloadFromEvaluation(result, breaker.State(), agentID)
mandate := ap2.PaymentMandateContents{
    Amount:      amount,
    Currency:    "USD",
    RiskPayload: riskPayload,
}
```

This gives issuers and payment networks visibility into agent risk assessments without requiring them to understand the governance internals.

### 3. Google ADK (via `adk-go`)

ADK's `adka2a.ExecutorConfig` exposes `BeforeExecuteCallback` and `AfterEventCallback`. Governance can integrate at this level too:

```go
executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
    BeforeExecuteCallback: func(ctx context.Context, reqCtx *a2asrv.RequestContext) (context.Context, error) {
        // Evaluate governance before agent runs
        result, err := breaker.Evaluate(ctx, extractFromReqCtx(reqCtx))
        if err != nil {
            return ctx, err
        }
        if !result.Allowed {
            return ctx, fmt.Errorf("blocked by governance: %s", result)
        }
        return ctx, nil
    },
})
```

## State Machine

```
                    ┌─────────┐
                    │ CLOSED  │ ← Normal operation
                    └────┬────┘
                         │ policy trips
                    ┌────▼────┐
            ┌──────→│  OPEN   │ ← Requests blocked
            │       └────┬────┘
            │            │ cooldown expires
            │       ┌────▼────┐
            │       │HALF_OPEN│ ← Probe requests
            │       └────┬────┘
            │           / \
            │     fail /   \ success
            │         /     \
            └────────┘       └──→ CLOSED

    OPEN ──[max consecutive failures]──→ TERMINATED
```

### State Transition Rules

1. **CLOSED → OPEN**: Any policy evaluation returns `Tripped = true`
2. **OPEN → HALF_OPEN**: `CooldownDuration` elapsed since last trip
3. **HALF_OPEN → CLOSED**: `ConsecutiveSuccessesToClose` successful probes
4. **HALF_OPEN → OPEN**: Any probe fails policy evaluation
5. **OPEN → TERMINATED**: `ConsecutiveFailuresToTerminate` consecutive trip cycles

## Thread Safety

`CircuitBreaker` is safe for concurrent use. All state access is protected by `sync.RWMutex`. Policies that maintain internal state (CumulativeSpend, Velocity, AnomalyDetection) have their own mutexes.

## Extensibility

### Custom Policies

Implement the `governance.Policy` interface:

```go
type CustomPolicy struct{}

func (p *CustomPolicy) Name() string { return "my_custom_policy" }
func (p *CustomPolicy) Evaluate(ctx context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
    // Custom logic
    return &governance.Evaluation{
        PolicyName: p.Name(),
        Reason:     governance.ReasonCustom,
    }, nil
}
```

### Custom State Stores

Implement `store.StateStore` for persistent storage (Redis, PostgreSQL, DynamoDB).

### Custom Request Extractors

Provide a `RequestExtractor` to the interceptor to extract domain-specific metadata from A2A requests.

## Security Review

The following security hardening patterns are applied throughout the codebase. Follow these when contributing new policies or state stores.

### NaN/Inf Input Validation

All value-based policies validate `TransactionValue` at the top of `Evaluate()`:

```go
if math.IsNaN(req.TransactionValue) || math.IsInf(req.TransactionValue, 0) {
    return &governance.Evaluation{Tripped: true, Message: "invalid transaction value: NaN or Inf"}, nil
}
```

Without this, `NaN > threshold` evaluates to `false` in IEEE 754, silently bypassing the policy.

### Division-by-Zero Guards

Score calculations guard against zero divisors:

```go
score := req.TransactionValue / p.MaxValue
if p.MaxValue == 0 {
    score = 1.0
}
```

### Bounded Memory Growth

All stateful policies cap internal data structures:

- `CumulativeSpend`: `maxSpendRecords = 100_000`
- `Velocity`: `maxVelocityTimestamps = 100_000`
- `MemoryStore`: `DefaultMaxEventsPerBreaker = 10_000`

When the cap is reached, oldest entries are dropped.

### Deadlock-Safe Escalation

The `CircuitBreaker` releases its mutex **before** calling `OnEscalation` or `OnTransition` handlers. This prevents deadlocks when handlers call back into the breaker:

```go
cb.mu.Unlock()
// Run escalation handler outside the lock to prevent deadlocks.
if cb.onEscalation != nil {
    _ = cb.onEscalation(ctx, cb, trippedEval)
}
```

### RiskLevel Score Handling

`RiskLevelFromScore()` clamps inputs to `[0.0, 1.0]` and treats `NaN` as `CRITICAL`.

## Dependencies

- `github.com/a2aproject/a2a-go` v0.3.10 — A2A Go SDK (interceptor package only)
- Go standard library — all other packages

The `governance/`, `policy/`, `store/`, and `ap2/` packages have **zero external dependencies** beyond the standard library, making them usable outside the A2A ecosystem.

## Future Work

- Redis/SQL state store implementations
- Prometheus/OpenTelemetry metrics export
- Multi-tenant governance (per-tenant circuit breakers)
- Policy hot-reload from configuration
- gRPC health check integration
- Dashboard UI for governance observability
