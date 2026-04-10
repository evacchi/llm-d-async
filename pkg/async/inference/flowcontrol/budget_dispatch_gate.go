/*
Copyright 2026 The llm-d Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flowcontrol

import (
	"context"
	"math"

	"sigs.k8s.io/controller-runtime/pkg/log"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"
)

// BudgetDispatchGate implements DispatchGate using the Dispatch Budget formula:
//
//	D = 1 - (F_SYS + F_EPP + B)
//
// where:
//   - F_SYS is the system saturation (fraction of system resources in use)
//   - F_EPP is the EPP virtual load (fraction of EPP queue depth vs system capacity)
//   - B is a configurable reserved baseline for burst protection
//
// Both F_SYS and F_EPP are expected to be in [0, 1] range. If eppSource is nil,
// F_EPP is treated as 0. The output budget is clamped to [0.0, 1.0].
//
// On error or missing data from sysSource, the gate returns the configured fallback.
// On error or missing data from eppSource, F_EPP is treated as 0 (soft failure).
type BudgetDispatchGate struct {
	sysSource MetricSource
	eppSource MetricSource // may be nil
	baseline  float64
	fallback  float64
}

// NewBudgetDispatchGate creates a new BudgetDispatchGate.
// sysSource provides the system saturation metric (F_SYS).
// eppSource provides the EPP virtual load metric (F_EPP); may be nil.
// baseline is the reserved fraction B (e.g. 0.05).
// fallback is the budget value returned on sysSource errors, clamped to [0, 1].
func NewBudgetDispatchGate(sysSource MetricSource, eppSource MetricSource, baseline float64, fallback float64) *BudgetDispatchGate {
	return &BudgetDispatchGate{
		sysSource: sysSource,
		eppSource: eppSource,
		baseline:  baseline,
		fallback:  math.Max(0.0, math.Min(1.0, fallback)),
	}
}

// Budget implements DispatchGate.
// On error or missing data from sysSource the gate returns the configured fallback.
// On error or missing data from eppSource, F_EPP is treated as 0.
// The output is always clamped to [0.0, 1.0].
func (g *BudgetDispatchGate) Budget(ctx context.Context) float64 {
	logger := log.FromContext(ctx)

	// Query F_SYS (required)
	sysSamples, err := g.sysSource.Query(ctx)
	if err != nil {
		logger.V(logutil.DEFAULT).Info("System metric source error, using fallback", "fallback", g.fallback, "error", err)
		return g.fallback
	}
	if len(sysSamples) == 0 {
		logger.V(logutil.DEFAULT).Info("No system saturation metrics, using fallback", "fallback", g.fallback)
		return g.fallback
	}
	fSys := sysSamples[0].Value
	if math.IsNaN(fSys) || math.IsInf(fSys, 0) {
		logger.V(logutil.DEFAULT).Info("Invalid system saturation value, using fallback", "fallback", g.fallback, "value", fSys)
		return g.fallback
	}

	// Query F_EPP (optional, soft failure)
	var fEpp float64
	if g.eppSource != nil {
		eppSamples, err := g.eppSource.Query(ctx)
		if err != nil {
			logger.V(logutil.DEFAULT).Info("EPP metric source error, treating F_EPP as 0", "error", err)
		} else if len(eppSamples) == 0 {
			logger.V(logutil.DEFAULT).Info("No EPP metrics, treating F_EPP as 0")
		} else {
			v := eppSamples[0].Value
			if math.IsNaN(v) || math.IsInf(v, 0) {
				logger.V(logutil.DEFAULT).Info("Invalid EPP metric value, treating F_EPP as 0", "value", v)
			} else {
				fEpp = v
			}
		}
	}

	// D = 1 - (F_SYS + F_EPP + B)
	d := 1.0 - (fSys + fEpp + g.baseline)

	return math.Min(1.0, math.Max(0.0, d))
}
