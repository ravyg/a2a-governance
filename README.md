# a2a-governance

Runtime governance middleware for [A2A (Agent-to-Agent)](https://github.com/a2aproject/A2A) protocol agents. Circuit breakers, spend limits, velocity controls, anomaly detection, and human escalation for autonomous agent transactions.

Plugs into any [`a2a-go`](https://github.com/a2aproject/a2a-go) server via interceptors with zero agent code changes.

## Why

AI agents are increasingly executing real transactions autonomously — purchasing, transferring funds, and interacting with external services. There is no standard way to govern this behavior at runtime. Static mandates check authority at signing time, but they don't address:

- An agent spending $50K in 10 minutes
- Anomalous purchasing patterns that deviate from normal behavior
- Transactions with untrusted or flagged vendors
- Operations outside business hours or authorized scope

**a2a-governance** fills this gap with a pluggable policy engine and circuit breaker that integrates natively with the A2A protocol.

## Installation

```bash
go get github.com/ravyg/a2a-governance
```

Requires Go 1.26+ and [`a2a-go`](https://github.com/a2aproject/a2a-go) v0.3.10+.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/ravyg/a2a-governance/governance"
    "github.com/ravyg/a2a-governance/policy"
)

func main() {
    breaker := governance.NewCircuitBreaker(governance.BreakerConfig{
        Policies: []governance.Policy{
            &policy.ValueThreshold{MaxValue: 10000},
            &policy.Velocity{MaxRequests: 100, Window: time.Minute},
            &policy.CumulativeSpend{MaxSpend: 50000, Window: 24 * time.Hour},
            &policy.VendorTrust{MinTrustScore: 0.7},
        },
        CooldownDuration: 30 * time.Second,
        OnEscalation: func(ctx context.Context, cb *governance.CircuitBreaker, eval *governance.Evaluation) error {
            fmt.Printf("ALERT: %s tripped — %s\n", eval.PolicyName, eval.Message)
            return nil
        },
    })

    result, err := breaker.Evaluate(context.Background(), &governance.RequestContext{
        TaskID:           "order-123",
        AgentID:          "shopping-agent",
        TransactionValue: 500.00,
        Currency:         "USD",
        VendorID:         "acme-store",
        Timestamp:        time.Now(),
    })
    if err != nil {
        fmt.Printf("Blocked: %v\n", err)
        return
    }
    fmt.Println(result) // ALLOWED (4 policies evaluated)
}
```

## Integrating with A2A Server

The package provides `CallInterceptor` and `RequestContextInterceptor` implementations that plug directly into `a2a-go`:

```go
import (
    "github.com/a2aproject/a2a-go/a2asrv"
    "github.com/ravyg/a2a-governance/governance"
    "github.com/ravyg/a2a-governance/interceptor"
    "github.com/ravyg/a2a-governance/policy"
)

breaker := governance.NewCircuitBreaker(governance.BreakerConfig{
    Policies: []governance.Policy{
        &policy.ValueThreshold{MaxValue: 5000},
        &policy.Velocity{MaxRequests: 50, Window: time.Hour},
    },
})

handler := a2asrv.NewHandler(
    myAgentExecutor,
    interceptor.WithGovernance(interceptor.GovernanceInterceptorConfig{
        Breaker:   breaker,
        BreakerID: "production",
    }),
    interceptor.WithRiskContext(breaker),
)
```

Every `message/send` and `message/stream` request is now evaluated against your policies before reaching the agent. No agent code changes required.

## Built-in Policies

| Policy | What it checks | Trip condition |
|--------|---------------|----------------|
| `ValueThreshold` | Single transaction value | Exceeds configured maximum |
| `CumulativeSpend` | Aggregate spend in time window | Total exceeds cap |
| `Velocity` | Request frequency | Rate exceeds limit |
| `AnomalyDetection` | Statistical deviation (Welford's) | Value is N std devs from mean |
| `VendorTrust` | Vendor/merchant trust score | Score below minimum threshold |
| `AuthorityScope` | Agent/vendor/currency allowlists | Request outside authorized scope |
| `TimeBased` | Time-of-day and day-of-week | Request outside allowed window |

## Circuit Breaker State Machine

```
CLOSED ──[policy trips]──> OPEN ──[cooldown]──> HALF_OPEN
                                                    │
                                              success│fail
                                                    │
                                              CLOSED  OPEN

OPEN ──[max consecutive failures]──> TERMINATED
```

- **CLOSED**: Normal operation, all requests evaluated
- **OPEN**: All requests blocked, human escalation triggered
- **HALF_OPEN**: Limited probe requests to test recovery
- **TERMINATED**: Permanent shutdown, manual reset required

## AP2 Integration

The `ap2/` package provides types from [AP2 Section 7.4 (Risk Signals)](https://github.com/google-agentic-commerce/AP2) for attaching governance risk assessments to payment mandates:

```go
import "github.com/ravyg/a2a-governance/ap2"

result, _ := breaker.Evaluate(ctx, reqCtx)
riskPayload := ap2.NewRiskPayloadFromEvaluation(result, breaker.State(), "my-agent")

mandate := ap2.CartMandate{
    MandateID:   "mandate-001",
    CartID:      "cart-42",
    TotalValue:  150.00,
    Currency:    "USD",
    RiskPayload: riskPayload,
}
```

## Custom Policies

Implement the `governance.Policy` interface:

```go
type MyPolicy struct{}

func (p *MyPolicy) Name() string { return "my_policy" }

func (p *MyPolicy) Evaluate(ctx context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
    // Your logic here
    return &governance.Evaluation{
        PolicyName: p.Name(),
        Reason:     governance.ReasonCustom,
    }, nil
}
```

## Package Structure

```
a2a-governance/
├── governance/     Core: CircuitBreaker, Policy interface, state machine
├── policy/         Built-in policies (7 implementations)
├── store/          State persistence (MemoryStore, StateStore interface)
├── interceptor/    a2a-go SDK integration (CallInterceptor, RiskContext)
├── ap2/            AP2 types (FCB, RiskPayload, mandates)
├── examples/
│   ├── basic/      Minimal circuit breaker demo
│   └── ecommerce/  Full ecommerce scenario with AP2 risk payloads
└── DESIGN.md       Architecture and integration documentation
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run with race detector
go test ./... -race

# Run a specific package
go test ./governance/...
```

## Running Examples

```bash
# Basic circuit breaker demo
go run ./examples/basic/

# Ecommerce scenario with AP2 risk payloads
go run ./examples/ecommerce/
```

## Security

This package has been security-reviewed for:

- NaN/Inf input validation to prevent policy bypass
- Bounded memory growth on all stateful policies
- Deadlock-safe escalation handlers (run outside mutex)
- Division-by-zero guards on score calculations

See the [Security Review section in DESIGN.md](DESIGN.md#security-review) for detailed patterns to follow when contributing.

## Roadmap

- [ ] Redis state store for multi-instance deployments
- [ ] Prometheus/OpenTelemetry metrics export
- [ ] Multi-tenant governance (per-tenant circuit breakers)
- [ ] Policy hot-reload from configuration
- [ ] GovernedExecutor (AgentExecutor wrapper)
- [ ] gRPC health check integration

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Whether you're fixing a bug, adding a policy, improving docs, or building a new state store — we'd love your help making agent governance better for everyone.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
