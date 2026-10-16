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

// moduleOutputs evaluates the module calls in dirPath and returns the
// `module` object their outputs are referenced through
// (module.<name>.<output>). Only local sources (./, ../) are followed. A
// call's inputs are evaluated against parent — so they may use var,
// data, path and literals, but not locals, which resolve later — and
// layered over the module's variable defaults. A call that can't be
// evaluated is left out, so references to it stay unresolved, as any
// other unresolvable expression does.
func moduleOutputs(ctx context.Context, dirPath string, parent *hcl.EvalContext, depth int) cty.Value {
	logger := logging.FromContext(ctx)
	calls, _ := scanBlocks(ctx, dirPath, moduleBlock)
	out := make(map[string]cty.Value, len(calls))
	for _, call := range calls {
		name, source, ok := localModuleSource(ctx, call)
		if !ok {
			continue
		}
		if depth >= maxModuleDepth {
			logger.Warnw("module calls nested too deep; skipping", "module", name, "dir", dirPath)
			continue
		}
		outputs, err := evalModule(ctx, filepath.Join(dirPath, source), moduleInputs(ctx, name, call, parent), parent, depth+1)
		if err != nil {
			logger.Warnw("cannot evaluate module", "module", name, "source", source, "error", err)
			continue
		}
		out[name] = outputs
	}
	return cty.ObjectVal(out)
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

// moduleInputs evaluates a module call's input attributes against parent;
// an input that doesn't resolve is left out, so the module's default (if
// any) applies.
func moduleInputs(ctx context.Context, name string, call *hclsyntax.Block, parent *hcl.EvalContext) map[string]cty.Value {
	inputs := make(map[string]cty.Value)
	for key, attr := range call.Body.Attributes {
		if moduleMetaArgs[key] {
			continue
		}
		v, diags := attr.Expr.Value(parent)
		if diags.HasErrors() || !v.IsWhollyKnown() {
			logging.FromContext(ctx).Debugw("module input unresolved", "module", name, "input", key, "errors", diags.Errs())
			continue
		}
		inputs[key] = v
	}
	return inputs
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
	evalCtx.Variables[moduleKey] = moduleOutputs(ctx, dir, &evalCtx, depth)
	resolveLocals(ctx, &evalCtx, cty.EmptyObjectVal, locals)

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
