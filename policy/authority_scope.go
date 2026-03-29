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

package policy

import (
	"context"
	"fmt"

	"github.com/ravyg/a2a-governance/governance"
)

// AuthorityScope trips when an agent attempts to operate outside its
// authorized scope. The scope is defined by a set of allowed agent IDs,
// vendor IDs, and/or currency codes.
type AuthorityScope struct {
	// AllowedAgentIDs restricts which agents may make requests.
	// Empty means all agents are allowed.
	AllowedAgentIDs []string
	// AllowedVendorIDs restricts which vendors the agent may transact with.
	// Empty means all vendors are allowed.
	AllowedVendorIDs []string
	// AllowedCurrencies restricts which currencies are permitted.
	// Empty means all currencies are allowed.
	AllowedCurrencies []string
	// MaxTransactionValue sets an authority-specific value cap (distinct from
	// ValueThreshold which is a general limit).
	MaxTransactionValue float64
}

var _ governance.Policy = (*AuthorityScope)(nil)

func (p *AuthorityScope) Name() string { return "authority_scope" }

func (p *AuthorityScope) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonAuthorityScope,
		Status:     governance.StatusPass,
	}

	if len(p.AllowedAgentIDs) > 0 && req.AgentID != "" {
		if !contains(p.AllowedAgentIDs, req.AgentID) {
			eval.Tripped = true
			eval.Status = governance.StatusFail
			eval.Score = 1.0
			eval.Message = fmt.Sprintf("agent %q is not in authorized scope", req.AgentID)
			return eval, nil
		}
	}

	if len(p.AllowedVendorIDs) > 0 && req.VendorID != "" {
		if !contains(p.AllowedVendorIDs, req.VendorID) {
			eval.Tripped = true
			eval.Status = governance.StatusFail
			eval.Score = 1.0
			eval.Message = fmt.Sprintf("vendor %q is not in authorized scope", req.VendorID)
			return eval, nil
		}
	}

	if len(p.AllowedCurrencies) > 0 && req.Currency != "" {
		if !contains(p.AllowedCurrencies, req.Currency) {
			eval.Tripped = true
			eval.Status = governance.StatusFail
			eval.Score = 1.0
			eval.Message = fmt.Sprintf("currency %q is not in authorized scope", req.Currency)
			return eval, nil
		}
	}

	if p.MaxTransactionValue > 0 && req.TransactionValue > p.MaxTransactionValue {
		eval.Tripped = true
		eval.Status = governance.StatusFail
		eval.Threshold = p.MaxTransactionValue
		eval.Actual = req.TransactionValue
		eval.Score = req.TransactionValue / p.MaxTransactionValue
		eval.Message = fmt.Sprintf("transaction value %.2f exceeds authority limit %.2f", req.TransactionValue, p.MaxTransactionValue)
		return eval, nil
	}

	return eval, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
