package solver

import (
	"fmt"
	"math"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
)

/* -------------------------------------------------------------------------- */
/*                          shared CEL environment                            */
/* -------------------------------------------------------------------------- */

var env *cel.Env

func init() {
	// declare the variables our expressions may reference
	declarations := cel.Declarations(
		decls.NewVar("cpu", decls.Double),
		decls.NewVar("ram", decls.Double),
		decls.NewVar("boot", decls.Double),
		decls.NewVar("x", decls.Double), // used only inside metric transforms
	)

	e, err := cel.NewEnv(declarations)
	if err != nil {
		panic(err)
	}
	env = e
}

/* -------------------------------------------------------------------------- */

type candidate struct {
	Node   *rcv1.RcNode
	Score  float64
	Detail map[string]float64
}

/* ------------------------ hard-constraint checker ------------------------- */

func satisfies(n *rcv1.RcNode, expr string) (bool, error) {
	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		return false, iss.Err()
	}
	prg, err := env.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := prg.Eval(toVars(n))
	if err != nil {
		return false, err
	}
	ok, _ := out.Value().(bool)
	return ok, nil
}

/* -------------------------- metric + transform ---------------------------- */

func metricValue(m rcv1.PolicyMetric, n *rcv1.RcNode) (float64, error) {
	var raw float64
	switch m.Key {
	case "cpu":
		raw = float64(n.Spec.CPU.Cores)
	case "ram":
		raw = float64(n.Spec.Memory)
	case "boot":
		raw = float64(n.Spec.BootSeconds)
	default:
		return 0, fmt.Errorf("unknown metric %q", m.Key)
	}

	if m.Transform == nil {
		return raw, nil
	}

	ast, iss := env.Parse(*m.Transform)
	if iss.Err() != nil {
		return 0, iss.Err()
	}
	prg, err := env.Program(ast)
	if err != nil {
		return 0, err
	}
	out, _, err := prg.Eval(map[string]interface{}{"x": raw})
	if err != nil {
		return 0, err
	}
	v, err := out.ConvertToNative(reflect.TypeOf(float64(0)))
	if err != nil {
		return 0, err
	}
	return v.(float64), nil
}

/* ----------------------------- public API --------------------------------- */

// PickBest returns the RcNode with the lowest weighted-score that satisfies
// *all* hard constraints in the supplied policy.
func PickBest(pol *rcv1.RcPolicy, nodes []rcv1.RcNode) (*rcv1.RcNode, error) {
	var best *candidate

outer:
	for i := range nodes {
		n := &nodes[i]

		// 1) hard constraints
		for _, hc := range pol.Spec.HardConstraints {
			ok, err := satisfies(n, hc.Expression)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue outer // reject this node
			}
		}

		// 2) weighted score
		cand := candidate{Node: n, Detail: map[string]float64{}}
		for _, m := range pol.Spec.Metrics {
			val, err := metricValue(m, n)
			if err != nil {
				return nil, err
			}
			// defensive: NaNs break comparisons
			if math.IsNaN(val) {
				val = math.Inf(1)
			}
			contrib := val * m.Weight
			cand.Detail[m.Key] = contrib
			cand.Score += contrib
		}

		if best == nil || cand.Score < best.Score {
			best = &cand
		}
	}
	if best == nil {
		return nil, nil
	}
	return best.Node, nil
}

/* ------------------------------- helpers ---------------------------------- */

func toVars(n *rcv1.RcNode) map[string]interface{} {
	return map[string]interface{}{
		"cpu":  float64(n.Spec.CPU.Cores),
		"ram":  float64(n.Spec.Memory),
		"boot": float64(n.Spec.BootSeconds),
	}
}
