package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"bindwitness/internal/witness"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintf(out, "bindwitness %s\n", witness.Version)
		return 0
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, "bindwitness capture|check --config contract.json --report report.json [--observation capture.json]\nbindwitness compare --config mapping.json --left a.json --right b.json --report report.json\nbindwitness --version")
		if len(args) == 0 {
			return 3
		}
		return 0
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	config := fs.String("config", "", "JSON contract / mapping")
	report := fs.String("report", "", "required output JSON file")
	observation := fs.String("observation", "", "saved observation for offline check")
	left := fs.String("left", "", "left observation")
	right := fs.String("right", "", "right observation")
	if e := fs.Parse(args[1:]); e != nil {
		return 3
	}
	if fs.NArg() != 0 || *config == "" || *report == "" {
		fmt.Fprintln(stderr, "INFRASTRUCTURE_ERROR: --config and --report required; unexpected positional arguments are rejected")
		return 3
	}
	infra := func(e error) int {
		r := witness.Result{SchemaVersion: 1, Kind: "error", Outcome: witness.InfrastructureError, BindingVerdict: witness.Unresolved, Findings: []witness.Finding{{ID: "HARNESS_ERROR", Message: e.Error()}}, Coverage: []witness.Coverage{}}
		if err := witness.WriteJSON(*report, r); err != nil {
			fmt.Fprintln(stderr, "INFRASTRUCTURE_ERROR: save report:", err)
		}
		fmt.Fprintln(stderr, "INFRASTRUCTURE_ERROR:", e)
		return 3
	}
	var result *witness.Result
	switch args[0] {
	case "capture", "check":
		if *left != "" || *right != "" || args[0] == "capture" && *observation != "" {
			return infra(fmt.Errorf("unsupported flags for %s", args[0]))
		}
		c, e := witness.LoadConfig(*config)
		if e != nil {
			return infra(e)
		}
		var o *witness.Observation
		if *observation != "" {
			o, e = witness.LoadObservation(*observation)
		} else {
			o, e = witness.Capture(c)
		}
		if e != nil {
			return infra(e)
		}
		if args[0] == "capture" {
			if e = witness.WriteJSON(*report, o); e != nil {
				return infra(e)
			}
			verdict := witness.Pass
			if !o.Capture.Complete {
				verdict = witness.Unresolved
			}
			fmt.Fprintf(out, "%s capture: %d bindings, workload exit %v, report %s\n", verdict, len(o.Bindings), exitText(o), *report)
			return witness.ExitCode(verdict)
		}
		result = witness.Evaluate(o, c)
	case "compare":
		if *left == "" || *right == "" || *observation != "" {
			return infra(fmt.Errorf("compare requires --left and --right"))
		}
		c, e := witness.LoadCompareConfig(*config)
		if e != nil {
			return infra(e)
		}
		a, e := witness.LoadObservation(*left)
		if e != nil {
			return infra(e)
		}
		b, e := witness.LoadObservation(*right)
		if e != nil {
			return infra(e)
		}
		result = witness.Compare(a, b, c)
	default:
		return infra(fmt.Errorf("unknown operation %q", args[0]))
	}
	if e := witness.WriteJSON(*report, result); e != nil {
		return infra(e)
	}
	fmt.Fprintf(out, "%s: report %s\n", result.Outcome, *report)
	// Concrete contract violations come before incompleteness in the terminal summary.
	first := -1
	for i, f := range result.Findings {
		if f.ID == "PROVIDER_NOT_ALLOWED" || f.ID == "PROVIDER_CHANGED" || f.ID == "ARTIFACT_PIN_MISMATCH" {
			first = i
			break
		}
	}
	if first < 0 && len(result.Findings) > 0 {
		first = 0
	}
	if first >= 0 {
		f := result.Findings[first]
		fmt.Fprintf(out, "%s: %s\n", f.ID, f.Message)
		showEvidence := func(label string, binding *witness.Binding, obs *witness.Observation) {
			if binding == nil || obs == nil {
				return
			}
			line := binding.TraceLine
			if line > 0 && line <= len(obs.Trace) {
				fmt.Fprintf(out, "%strace[%d]: %s\n", label, line, obs.Trace[line-1])
			}
		}
		obs := result.Observation
		if obs == nil {
			obs = result.Left
		}
		showEvidence("", f.Witness, obs)
		showEvidence("right ", f.OtherWitness, result.Right)
	}
	return witness.ExitCode(result.Outcome)
}
func exitText(o *witness.Observation) any {
	if o.Workload.ExitCode == nil {
		return "not started"
	}
	return *o.Workload.ExitCode
}
