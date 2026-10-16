package terraform

import (
	"context"
	"maps"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/jingle2008/toolkit/internal/fileutil"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
)

const (
	moduleBlock = "module"
	// maxModuleDepth bounds nested module calls, so a cycle of local
	// sources can't recurse forever.
	maxModuleDepth = 8
)

// moduleMetaArgs are a module block's attributes that aren't inputs.
var moduleMetaArgs = map[string]bool{
	"source": true, "version": true, "providers": true,
	"count": true, "for_each": true, "depends_on": true,
}

// resolveLocalsAndModules resolves attributes (dirPath's locals) together
// with dirPath's module calls, which may feed each other: a local can read
// module.<name>.<output>, and a call's inputs can read local.<name> or
// another call's outputs. It alternates — evaluate every call whose
// inputs are ready, then resolve locals — until a round settles
// neither a call nor a local, keeping evalCtx's `module` object current. Only local sources
// (./, ../) are followed. A call's inputs are layered over the module's
// variable defaults; an input naming a local or module that hasn't
// resolved yet defers the whole call rather than falling back to the
// default, so no output is computed from a stand-in value. A call that never becomes ready, or can't be evaluated,
// is left out, so references to it stay unresolved, as any other
// unresolvable expression does. Returns the final `local` object.
func resolveLocalsAndModules(
	ctx context.Context,
	dirPath string,
	evalCtx *hcl.EvalContext,
	localObject cty.Value,
	attributes hclsyntax.Attributes,
	depth int,
) cty.Value {
	calls := moduleCalls(ctx, dirPath, depth)
	outputs := make(map[string]cty.Value, len(calls))
	evalCtx.Variables[moduleKey] = cty.EmptyObjectVal
	for {
		progress := false
		for name, call := range calls {
			inputs, ready := moduleInputs(ctx, name, call.block, evalCtx)
			if !ready {
				continue
			}
			delete(calls, name)
			progress = true
			out, err := evalModule(ctx, filepath.Join(dirPath, call.source), inputs, evalCtx, depth+1)
			if err != nil {
				logging.FromContext(ctx).Warnw("cannot evaluate module", "module", name, "source", call.source, "error", err)
				continue
			}
			outputs[name] = out
			evalCtx.Variables[moduleKey] = cty.ObjectVal(outputs)
		}
		pending := len(attributes)
		localObject = resolveLocals(ctx, evalCtx, localObject, attributes)
		if len(attributes) < pending {
			progress = true
		}
		if !progress || len(calls) == 0 {
			break
		}
	}
	for name := range calls {
		logging.FromContext(ctx).Debugw("module inputs never resolved; skipping", "module", name, "dir", dirPath)
	}
	return localObject
}

type moduleCall struct {
	block  *hclsyntax.Block
	source string
}

// moduleCalls returns dirPath's local-source module calls by name, or
// none once depth reaches maxModuleDepth.
func moduleCalls(ctx context.Context, dirPath string, depth int) map[string]moduleCall {
	blocks, _ := scanBlocks(ctx, dirPath, moduleBlock)
	calls := make(map[string]moduleCall, len(blocks))
	for _, b := range blocks {
		name, source, ok := localModuleSource(ctx, b)
		if !ok {
			continue
		}
		if depth >= maxModuleDepth {
			logging.FromContext(ctx).Warnw("module calls nested too deep; skipping", "module", name, "dir", dirPath)
			continue
		}
		calls[name] = moduleCall{block: b, source: source}
	}
	return calls
}

// localModuleSource returns a module call's name and its source when the
// source is a local path; ok is false otherwise.
func localModuleSource(ctx context.Context, call *hclsyntax.Block) (name, source string, ok bool) {
	if len(call.Labels) == 0 {
		return "", "", false
	}
	name = call.Labels[0]
	src, ok := call.Body.Attributes["source"]
	if !ok {
		return "", "", false
	}
	v, diags := src.Expr.Value(nil)
	if diags.HasErrors() || v.Type() != cty.String {
		return "", "", false
	}
	source = v.AsString()
	if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
		logging.FromContext(ctx).Debugw("skipping non-local module", "module", name, "source", source)
		return "", "", false
	}
	return name, source, true
}

// moduleInputs evaluates a module call's input attributes against parent.
// An input that doesn't resolve is left out, so the module's default (if
// any) applies — unless it names a local or module, which may still
// resolve: then ready is false and the call should wait.
func moduleInputs(ctx context.Context, name string, call *hclsyntax.Block, parent *hcl.EvalContext) (inputs map[string]cty.Value, ready bool) {
	inputs = make(map[string]cty.Value)
	for key, attr := range call.Body.Attributes {
		if moduleMetaArgs[key] {
			continue
		}
		v, diags := attr.Expr.Value(parent)
		if diags.HasErrors() || !v.IsWhollyKnown() {
			if referencesPending(attr.Expr) {
				return nil, false
			}
			logging.FromContext(ctx).Debugw("module input unresolved", "module", name, "input", key, "errors", diags.Errs())
			continue
		}
		inputs[key] = v
	}
	return inputs, true
}

// referencesPending reports whether expr reads a local or a module
// output — values resolveLocalsAndModules may not have resolved yet.
func referencesPending(expr hclsyntax.Expression) bool {
	for _, tr := range expr.Variables() {
		if root := tr.RootName(); root == localKey || root == moduleKey {
			return true
		}
	}
	return false
}

// evalModule evaluates the module in dir — its locals, then its
// outputs, with inputs over its variable defaults — and returns its
// resolved outputs as an object. The module sees parent's data sources,
// its own path.module, and its own module calls. Locals and outputs are
// kept apart (unlike LoadLocalAttributes): a module commonly exports a
// local under its own name, output "x" { value = local.x }.
func evalModule(ctx context.Context, dir string, inputs map[string]cty.Value, parent *hcl.EvalContext, depth int) (cty.Value, error) {
	localBlocks, err := scanBlocks(ctx, dir, localBlock)
	if err != nil {
		return cty.NilVal, err
	}
	outputs, err := scanBlocks(ctx, dir, outputBlock)
	if err != nil {
		return cty.NilVal, err
	}
	locals := make(hclsyntax.Attributes)
	for _, b := range localBlocks {
		maps.Copy(locals, b.Body.Attributes)
	}
	vars, _ := getVariableDefaults(ctx, dir)
	maps.Copy(vars, inputs)
	evalCtx := hcl.EvalContext{
		Variables: map[string]cty.Value{
			localKey: cty.EmptyObjectVal,
			varKey:   cty.ObjectVal(vars),
			pathKey:  cty.ObjectVal(map[string]cty.Value{"module": cty.StringVal(dir)}),
		},
		Functions: localFuncMap,
	}
	if data, ok := parent.Variables[dataKey]; ok {
		evalCtx.Variables[dataKey] = data
	}
	resolveLocalsAndModules(ctx, dir, &evalCtx, cty.EmptyObjectVal, locals, depth)

	logger := logging.FromContext(ctx)
	out := make(map[string]cty.Value, len(outputs))
	for _, o := range outputs {
		attr, ok := o.Body.Attributes[outputValue]
		if len(o.Labels) == 0 || !ok {
			continue
		}
		v, diags := attr.Expr.Value(&evalCtx)
		if diags.HasErrors() || !v.IsWhollyKnown() {
			logger.Debugw("module output unresolved", "dir", dir, "output", o.Labels[0], "errors", diags.Errs())
			continue
		}
		out[o.Labels[0]] = v
	}
	return cty.ObjectVal(out), nil
}

// scanBlocks returns the top-level blocks of type blockType in dir's .tf
// files. Files that don't parse are skipped; LoadLocalAttributes reports
// them.
func scanBlocks(ctx context.Context, dir, blockType string) ([]*hclsyntax.Block, error) {
	files, err := fileutil.ListFiles(ctx, dir, ".tf")
	if err != nil {
		return nil, err
	}
	var out []*hclsyntax.Block
	for _, f := range files {
		file, diags := hclparse.NewParser().ParseHCLFile(f)
		if diags.HasErrors() {
			continue
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, b := range body.Blocks {
			if b.Type == blockType {
				out = append(out, b)
			}
		}
	}
	return out, nil
}
