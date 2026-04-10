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
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// Core formula tests: D = 1 - (F_SYS + F_EPP + B)

func TestBudgetDispatchGate_CoreFormula(t *testing.T) {
	// Example from design doc: F_SYS=0.5, F_EPP=0.1, B=0.05 → D=0.35
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		&mockMetricSource{samples: []Sample{{Value: 0.1}}},
		0.05, 0.0,
	)
	require.InDelta(t, 0.35, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_ZeroLoad(t *testing.T) {
	// F_SYS=0, F_EPP=0, B=0.05 → D=0.95
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.0}}},
		&mockMetricSource{samples: []Sample{{Value: 0.0}}},
		0.05, 0.0,
	)
	require.InDelta(t, 0.95, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_Overloaded(t *testing.T) {
	// F_SYS=0.8, F_EPP=0.2, B=0.05 → D=-0.05 → clamped to 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.8}}},
		&mockMetricSource{samples: []Sample{{Value: 0.2}}},
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_FullSaturation(t *testing.T) {
	// F_SYS=1.0, F_EPP=0.0, B=0.05 → D=-0.05 → clamped to 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 1.0}}},
		&mockMetricSource{samples: []Sample{{Value: 0.0}}},
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_ZeroBaseline(t *testing.T) {
	// F_SYS=0.3, F_EPP=0.2, B=0.0 → D=0.5
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.3}}},
		&mockMetricSource{samples: []Sample{{Value: 0.2}}},
		0.0, 0.0,
	)
	require.InDelta(t, 0.5, gate.Budget(context.Background()), 1e-9)
}

// EPP source nil/error tests (soft failure)

func TestBudgetDispatchGate_NilEppSource(t *testing.T) {
	// F_SYS=0.5, no EPP source, B=0.05 → D = 1-(0.5+0+0.05) = 0.45
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		nil,
		0.05, 0.0,
	)
	require.InDelta(t, 0.45, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_EppSourceError(t *testing.T) {
	// EPP error → F_EPP treated as 0; D = 1-(0.5+0+0.05) = 0.45
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		&mockMetricSource{err: errors.New("connection refused")},
		0.05, 0.0,
	)
	require.InDelta(t, 0.45, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_EppEmptySamples(t *testing.T) {
	// EPP returns no samples → F_EPP treated as 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		&mockMetricSource{samples: []Sample{}},
		0.05, 0.0,
	)
	require.InDelta(t, 0.45, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_EppNaN(t *testing.T) {
	// EPP returns NaN → F_EPP treated as 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		&mockMetricSource{samples: []Sample{{Value: math.NaN()}}},
		0.05, 0.0,
	)
	require.InDelta(t, 0.45, gate.Budget(context.Background()), 1e-9)
}

func TestBudgetDispatchGate_EppInf(t *testing.T) {
	// EPP returns Inf → F_EPP treated as 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		&mockMetricSource{samples: []Sample{{Value: math.Inf(1)}}},
		0.05, 0.0,
	)
	require.InDelta(t, 0.45, gate.Budget(context.Background()), 1e-9)
}

// SYS source error tests (hard failure → fallback)

func TestBudgetDispatchGate_SysError_FailClosed(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{err: errors.New("connection refused")},
		&mockMetricSource{samples: []Sample{{Value: 0.1}}},
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_SysError_FailOpen(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{err: errors.New("connection refused")},
		&mockMetricSource{samples: []Sample{{Value: 0.1}}},
		0.05, 1.0,
	)
	require.Equal(t, 1.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_SysEmptySamples(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{}},
		&mockMetricSource{samples: []Sample{{Value: 0.1}}},
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_SysNaN(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: math.NaN()}}},
		nil,
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_SysInf(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: math.Inf(1)}}},
		nil,
		0.05, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

// Clamping tests

func TestBudgetDispatchGate_NegativeBudgetClampedToZero(t *testing.T) {
	// F_SYS=0.9, F_EPP=0.5, B=0.1 → D=1-1.5=-0.5 → clamped to 0
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: 0.9}}},
		&mockMetricSource{samples: []Sample{{Value: 0.5}}},
		0.1, 0.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_BudgetAboveOneClampedToOne(t *testing.T) {
	// F_SYS=-0.5, F_EPP=0.0, B=0.0 → D=1-(-0.5)=1.5 → clamped to 1
	gate := NewBudgetDispatchGate(
		&mockMetricSource{samples: []Sample{{Value: -0.5}}},
		&mockMetricSource{samples: []Sample{{Value: 0.0}}},
		0.0, 0.0,
	)
	require.Equal(t, 1.0, gate.Budget(context.Background()))
}

// Fallback clamping at construction time

func TestBudgetDispatchGate_FallbackClampedAboveOne(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{err: errors.New("error")},
		nil,
		0.05, 2.0,
	)
	require.Equal(t, 1.0, gate.Budget(context.Background()))
}

func TestBudgetDispatchGate_FallbackClampedBelowZero(t *testing.T) {
	gate := NewBudgetDispatchGate(
		&mockMetricSource{err: errors.New("error")},
		nil,
		0.05, -1.0,
	)
	require.Equal(t, 0.0, gate.Budget(context.Background()))
}
