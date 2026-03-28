// Copyright 2026 Ravish Gupta
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interceptor

import (
	"context"
	"fmt"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"

	"github.com/ravyg/a2a-governance/governance"
	"github.com/ravyg/a2a-governance/store"
)

type governanceResultKey struct{}

// GovernanceResultFromContext retrieves the governance EvaluationResult
// attached to a context by the governance interceptor.
func GovernanceResultFromContext(ctx context.Context) (*governance.EvaluationResult, bool) {
	result, ok := ctx.Value(governanceResultKey{}).(*governance.EvaluationResult)
	return result, ok
}

// RequestExtractor extracts governance-relevant metadata from an A2A request.
// If nil, the default extractor is used.
type RequestExtractor func(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) *governance.RequestContext

// GovernanceInterceptorConfig configures the governance CallInterceptor.
type GovernanceInterceptorConfig struct {
	// Breaker is the circuit breaker to use for governance decisions.
	Breaker *governance.CircuitBreaker
	// Store is optional; if provided, governance events are recorded for audit.
	Store store.StateStore
	// BreakerID is the identifier used for storing state. Defaults to "default".
	BreakerID string
	// Extractor extracts request metadata for policy evaluation.
	// If nil, the default extractor is used.
	Extractor RequestExtractor
	// Methods lists the A2A methods to intercept. If empty, only "message/send"
	// and "message/stream" are intercepted.
	Methods []string
}

// GovernanceCallInterceptor implements a2asrv.CallInterceptor and evaluates
// governance policies on incoming A2A requests.
type GovernanceCallInterceptor struct {
	a2asrv.PassthroughCallInterceptor
	config GovernanceInterceptorConfig
}

var _ a2asrv.CallInterceptor = (*GovernanceCallInterceptor)(nil)

// NewGovernanceInterceptor creates a CallInterceptor that evaluates governance
// policies on A2A requests.
func NewGovernanceInterceptor(config GovernanceInterceptorConfig) *GovernanceCallInterceptor {
	if config.BreakerID == "" {
		config.BreakerID = "default"
	}
	if len(config.Methods) == 0 {
		config.Methods = []string{"OnSendMessage", "OnSendMessageStream"}
	}
	return &GovernanceCallInterceptor{config: config}
}

func (g *GovernanceCallInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) (context.Context, error) {
	// Only intercept configured methods.
	if !g.shouldIntercept(callCtx.Method()) {
		return ctx, nil
	}

	// Extract request context.
	var reqCtx *governance.RequestContext
	if g.config.Extractor != nil {
		reqCtx = g.config.Extractor(ctx, callCtx, req)
	} else {
		reqCtx = g.defaultExtract(ctx, callCtx, req)
	}

	if reqCtx == nil {
		return ctx, nil
	}

	// Evaluate policies via circuit breaker.
	result, err := g.config.Breaker.Evaluate(ctx, reqCtx)
	if err != nil {
		return ctx, fmt.Errorf("governance evaluation failed: %w", err)
	}

	// Record the event if a store is configured.
	if g.config.Store != nil {
		event := &store.GovernanceEvent{
			ID:               fmt.Sprintf("%s-%d", reqCtx.TaskID, time.Now().UnixNano()),
			BreakerID:        g.config.BreakerID,
			Timestamp:        time.Now(),
			State:            g.config.Breaker.State(),
			Result:           result,
			TaskID:           reqCtx.TaskID,
			AgentID:          reqCtx.AgentID,
			UserID:           reqCtx.UserID,
			TransactionValue: reqCtx.TransactionValue,
		}
		if storeErr := g.config.Store.RecordEvent(ctx, event); storeErr != nil {
			// Log but don't block on store failures.
			_ = storeErr
		}
	}

	// Attach result to context for downstream consumers.
	ctx = context.WithValue(ctx, governanceResultKey{}, result)

	if !result.Allowed {
		return ctx, fmt.Errorf("request blocked by governance: %s", result)
	}

	return ctx, nil
}

func (g *GovernanceCallInterceptor) shouldIntercept(method string) bool {
	for _, m := range g.config.Methods {
		if m == method {
			return true
		}
	}
	return false
}

func (g *GovernanceCallInterceptor) defaultExtract(_ context.Context, _ *a2asrv.CallContext, req *a2asrv.Request) *governance.RequestContext {
	if req == nil || req.Payload == nil {
		return nil
	}

	reqCtx := &governance.RequestContext{
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}

	// Extract from MessageSendParams if available.
	if params, ok := req.Payload.(*a2a.MessageSendParams); ok {
		if params.Message != nil {
			reqCtx.TaskID = params.Message.ID
		}
		if params.Config != nil {
			reqCtx.Metadata["blocking"] = params.Config.Blocking
		}
	}

	return reqCtx
}

// WithGovernance returns a RequestHandlerOption that adds governance
// interception to an A2A server handler.
func WithGovernance(config GovernanceInterceptorConfig) a2asrv.RequestHandlerOption {
	return a2asrv.WithCallInterceptor(NewGovernanceInterceptor(config))
}
