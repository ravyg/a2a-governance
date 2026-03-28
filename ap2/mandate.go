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

package ap2

// IntentMandate represents the AP2 intent-level mandate with risk payload.
type IntentMandate struct {
	// MandateID is the unique mandate identifier.
	MandateID string `json:"mandate_id"`
	// Intent describes the intended action.
	Intent string `json:"intent"`
	// RiskPayload carries the governance risk assessment.
	RiskPayload *RiskPayload `json:"risk_payload,omitempty"`
}

// CartMandate represents the AP2 cart-level mandate with risk payload.
type CartMandate struct {
	// MandateID is the unique mandate identifier.
	MandateID string `json:"mandate_id"`
	// CartID references the cart being governed.
	CartID string `json:"cart_id"`
	// TotalValue is the cart's total monetary value.
	TotalValue float64 `json:"total_value"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// RiskPayload carries the governance risk assessment.
	RiskPayload *RiskPayload `json:"risk_payload,omitempty"`
}

// PaymentMandateContents represents AP2 payment mandate contents with risk payload.
type PaymentMandateContents struct {
	// MandateID is the unique mandate identifier.
	MandateID string `json:"mandate_id"`
	// PaymentMethod describes the payment instrument.
	PaymentMethod string `json:"payment_method"`
	// Amount is the payment amount.
	Amount float64 `json:"amount"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// VendorID identifies the merchant/vendor.
	VendorID string `json:"vendor_id,omitempty"`
	// RiskPayload carries the governance risk assessment.
	RiskPayload *RiskPayload `json:"risk_payload,omitempty"`
}
