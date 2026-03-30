# Contributing to a2a-governance

Thank you for your interest in contributing to a2a-governance! This project aims to make runtime governance a standard practice for autonomous AI agents. Every contribution helps make agent systems safer and more predictable.

## Code of Conduct

We are committed to providing a welcoming and inclusive experience for everyone. By participating in this project, you agree to abide by the following principles:

- **Be respectful.** Treat everyone with dignity and respect. Disagreements are fine; personal attacks are not.
- **Be constructive.** Offer actionable feedback. If you see a problem, suggest a solution.
- **Be inclusive.** Welcome newcomers. Explain context rather than assuming everyone has it.
- **Be collaborative.** This is open source — we're building together. Assume good intent.
- **Be professional.** Harassment, discrimination, and abusive behavior of any kind will not be tolerated.

If you experience or witness unacceptable behavior, please open an issue or contact the maintainers directly.

## How to Contribute

### Reporting Issues

- Check if the issue already exists before opening a new one
- Include Go version, OS, and a minimal reproduction if possible
- Use the issue templates when available

### Suggesting Features

- Open an issue with the `[Feature]` prefix in the title
- Describe the use case and why existing functionality doesn't cover it
- If you're proposing a new policy, include the trip condition logic and examples

### Submitting Pull Requests

1. **Fork** the repository and create a branch from `main`
2. **Write tests** for any new functionality
3. **Run the full test suite** before submitting:
   ```bash
   go test ./... -race
   go vet ./...
   ```
4. **Keep PRs focused** — one feature or fix per PR
5. **Write clear commit messages** that explain *why*, not just *what*
6. Open the PR and fill in the template

### What We're Looking For

Here are some areas where contributions are especially welcome:

#### New Policies

We already ship 7 policies (ValueThreshold, CumulativeSpend, Velocity, AnomalyDetection, VendorTrust, AuthorityScope, TimeBased). Ideas for new ones:

- Geographic restrictions (geo-fencing agent operations)
- Budget period policies (monthly/quarterly budget enforcement)
- Merchant category code (MCC) restrictions
- Multi-party approval workflows
- ML-based anomaly detection integrations

#### State Store Implementations
- Redis (with Lua scripts for atomic operations)
- PostgreSQL / MySQL
- DynamoDB
- etcd (for Kubernetes-native deployments)

#### Observability
- Prometheus metrics collector
- OpenTelemetry integration
- Structured logging middleware

#### Documentation
- Tutorials and how-to guides
- Integration examples with popular agent frameworks
- Architecture decision records (ADRs)

#### Testing
- Fuzz testing for policy evaluation
- Benchmark tests for high-throughput scenarios
- Integration tests with real a2a-go servers

## Development Setup

### Prerequisites

- Go 1.26 or later
- Git

### Getting Started

```bash
# Clone your fork
git clone git@github.com:YOUR_USERNAME/a2a-governance.git
cd a2a-governance

# Install dependencies
go mod download

# Run tests
go test ./...

# Run examples
go run ./examples/basic/
go run ./examples/ecommerce/

# Run with race detector
go test ./... -race

# Run vet
go vet ./...
```

### Project Structure

```
governance/     Core types and circuit breaker (no external deps)
policy/         Built-in policy implementations
store/          State persistence interfaces and implementations
interceptor/    a2a-go SDK integration layer
ap2/            AP2 protocol-specific types
examples/       Runnable example programs
```

### Key Interfaces

When contributing, these are the primary extension points:

```go
// Implement this to add a new governance policy
type Policy interface {
    Name() string
    Evaluate(ctx context.Context, req *RequestContext) (*Evaluation, error)
}

// Implement this to add a new state store
type StateStore interface {
    SaveSnapshot(ctx context.Context, snapshot *BreakerSnapshot) error
    LoadSnapshot(ctx context.Context, breakerID string) (*BreakerSnapshot, error)
    RecordEvent(ctx context.Context, event *GovernanceEvent) error
    ListEvents(ctx context.Context, breakerID string, limit int) ([]*GovernanceEvent, error)
}
```

## Coding Guidelines

### Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use table-driven tests where appropriate
- Keep functions short and focused
- Export only what needs to be exported

### Testing

- Every new policy must have tests covering: normal pass, trip condition, edge cases
- Use `t.Helper()` in test helpers
- Test concurrent safety with `-race` flag
- Aim for meaningful coverage, not 100% line coverage

### Security

- Validate all numeric inputs for NaN/Inf
- Guard against division by zero in score calculations
- Bound all internal data structures to prevent memory exhaustion
- Never call external callbacks (escalation handlers, hooks) while holding locks
- See the security review in [DESIGN.md](DESIGN.md) for patterns to follow

### Documentation

- All exported types and functions must have godoc comments
- Include usage examples in doc comments for key APIs
- Update DESIGN.md if your change affects architecture

## Review Process

1. A maintainer will review your PR, usually within a few days
2. We may ask for changes — this is normal and collaborative
3. Once approved, a maintainer will merge your PR
4. Your contribution will be included in the next release

## Becoming a Maintainer

Regular contributors who demonstrate good judgment and sustained engagement may be invited to become maintainers. There's no fixed threshold — we look for:

- Quality contributions over time
- Constructive code reviews on others' PRs
- Helping other contributors in issues and discussions
- Understanding of the project's goals and architecture

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

## Questions?

Open an issue with the `[Question]` tag, or start a discussion. We're happy to help you get started.
