package terraform

import (
	"cmp"
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/jingle2008/toolkit/internal/fileutil"
	logging "github.com/jingle2008/toolkit/internal/logging"
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
// another call's outputs. Only local sources (./, ../) are followed.
//
// A local or call is evaluated only once every local and call it reads has
// settled (resolved, or found unresolvable), so try(), can() and lookup()
// never settle on a fallback for a value that is merely not ready yet. A
// call's inputs are layered over the module's variable defaults: an input
// that doesn't resolve falls back to the default. When nothing is ready but
// something is pending, the references form a cycle; it is broken by
// evaluating one item against what has resolved so far.
//
// It deletes every local it resolves from attributes, so whatever remains
// could not be evaluated, and keeps evalCtx's `local` and `module` objects
// current. Returns the final `local` object.
func resolveLocalsAndModules(
	ctx context.Context,
	dirPath string,
	evalCtx *hcl.EvalContext,
	localObject cty.Value,
	attributes hclsyntax.Attributes,
	depth int,
) cty.Value {
	r := &resolver{
		dirPath: dirPath, evalCtx: evalCtx, depth: depth,
		local:   localObject,
		locals:  maps.Clone(attributes),
		calls:   moduleCalls(ctx, dirPath, depth),
		outputs: make(map[string]cty.Value),
	}
	evalCtx.Variables[localKey] = localObject
	evalCtx.Variables[moduleKey] = cty.EmptyObjectVal
	for r.resolveReady(ctx) || r.breakCycle(ctx) {
	}
	for key := range attributes {
		if r.resolved[key] {
			delete(attributes, key)
		}
	}
	return r.local
}

// resolver holds resolveLocalsAndModules' state: the locals and calls not
// yet settled, and what has resolved.
type resolver struct {
	dirPath  string
	evalCtx  *hcl.EvalContext
	depth    int
	local    cty.Value
	locals   hclsyntax.Attributes  // unsettled locals
	calls    map[string]moduleCall // unsettled calls
	outputs  map[string]cty.Value
	resolved map[string]bool // locals that resolved
}

// resolveReady settles every local and call whose references have all
// settled, until none is left; it reports whether it settled any.
func (r *resolver) resolveReady(ctx context.Context) bool {
	settled := false
	for progress := true; progress; {
		progress = false
		for _, key := range r.localOrder() {
			if !r.readsPending(r.locals[key].Expr) {
				r.settleLocal(key)
				progress = true
			}
		}
		for _, name := range slices.Sorted(maps.Keys(r.calls)) {
			if !r.callReadsPending(r.calls[name]) {
				r.settleCall(ctx, name)
				progress = true
			}
		}
		settled = settled || progress
	}
	return settled
}

// breakCycle settles one item of a reference cycle against what has
// resolved so far: the first local (fewest references first) that
// resolves that way, else the first call. With only unresolvable locals
// left, it settles them all and reports false.
func (r *resolver) breakCycle(ctx context.Context) bool {
	for _, key := range r.localOrder() {
		if v, ok := r.evalLocal(key); ok {
			logging.FromContext(ctx).Debug("breaking a reference cycle", "local", key, "dir", r.dirPath)
			r.setLocal(key, v)
			return true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(r.calls)) {
		logging.FromContext(ctx).Debug("breaking a reference cycle", "module", name, "dir", r.dirPath)
		r.settleCall(ctx, name)
		return true
	}
	clear(r.locals)
	return false
}

// localOrder returns the unsettled locals, fewest references first, then
// by name.
func (r *resolver) localOrder() []string {
	keys := slices.Collect(maps.Keys(r.locals))
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(
			cmp.Compare(len(r.locals[a].Expr.Variables()), len(r.locals[b].Expr.Variables())),
			cmp.Compare(a, b),
		)
	})
	return keys
}

func (r *resolver) evalLocal(key string) (cty.Value, bool) {
	v, diags := r.locals[key].Expr.Value(r.evalCtx)
	return v, !diags.HasErrors() && v.IsWhollyKnown()
}

// settleLocal evaluates a local; one that doesn't resolve is settled as
// unresolvable (its references have all settled, so it never will).
func (r *resolver) settleLocal(key string) {
	if v, ok := r.evalLocal(key); ok {
		r.setLocal(key, v)
		return
	}
	delete(r.locals, key)
}

func (r *resolver) setLocal(key string, v cty.Value) {
	delete(r.locals, key)
	r.local = mergeObject(r.local, key, v)
	r.evalCtx.Variables[localKey] = r.local
	if r.resolved == nil {
		r.resolved = make(map[string]bool)
	}
	r.resolved[key] = true
}

// settleCall evaluates a module call with its inputs as they resolve now.
func (r *resolver) settleCall(ctx context.Context, name string) {
	call := r.calls[name]
	delete(r.calls, name)
	inputs := moduleInputs(ctx, name, call.block, r.evalCtx)
	out, err := evalModule(ctx, filepath.Join(r.dirPath, call.source), inputs, r.evalCtx, r.depth+1)
	if err != nil {
		logging.FromContext(ctx).Warn("cannot evaluate module", "module", name, "module_source", call.source, "error", err)
		return
	}
	r.outputs[name] = out
	r.evalCtx.Variables[moduleKey] = cty.ObjectVal(r.outputs)
}

// readsPending reports whether expr reads an unsettled local or call. A
// reference that doesn't name one (local["x"]) counts as pending while
// any of its kind is.
func (r *resolver) readsPending(expr hclsyntax.Expression) bool {
	for _, tr := range expr.Variables() {
		var name string
		if len(tr) > 1 {
			if a, ok := tr[1].(hcl.TraverseAttr); ok {
				name = a.Name
			}
		}
		switch tr.RootName() {
		case localKey:
			if _, ok := r.locals[name]; ok || (name == "" && len(r.locals) > 0) {
				return true
			}
		case moduleKey:
			if _, ok := r.calls[name]; ok || (name == "" && len(r.calls) > 0) {
				return true
			}
		}
	}
	return false
}

func (r *resolver) callReadsPending(call moduleCall) bool {
	for key, attr := range call.block.Body.Attributes {
		if !moduleMetaArgs[key] && r.readsPending(attr.Expr) {
			return true
		}
	}
	return false
}

type moduleCall struct {
	block  *hclsyntax.Block
	source string
}

// moduleCalls returns dirPath's local-source module calls by name, or
// none once depth reaches maxModuleDepth.
func moduleCalls(ctx context.Context, dirPath string, depth int) map[string]moduleCall {
	blocks, err := scanBlocks(ctx, dirPath, moduleBlock)
	if err != nil {
		logging.FromContext(ctx).Warn("can't scan module calls", "dir", dirPath, "error", err)
	}
	calls := make(map[string]moduleCall, len(blocks))
	for _, b := range blocks {
		name, source, ok := localModuleSource(ctx, b)
		if !ok {
			continue
		}
		if depth >= maxModuleDepth {
			logging.FromContext(ctx).Warn("module calls nested too deep; skipping", "module", name, "dir", dirPath)
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
		logging.FromContext(ctx).Debug("skipping non-local module", "module", name, "module_source", source)
		return "", "", false
	}
	return name, source, true
}

// moduleInputs evaluates a module call's input attributes against parent.
// An input that doesn't resolve is left out, so the module's default (if
// any) applies.
func moduleInputs(ctx context.Context, name string, call *hclsyntax.Block, parent *hcl.EvalContext) map[string]cty.Value {
	inputs := make(map[string]cty.Value)
	for key, attr := range call.Body.Attributes {
		if moduleMetaArgs[key] {
			continue
		}
		v, diags := attr.Expr.Value(parent)
		if diags.HasErrors() || !v.IsWhollyKnown() {
			logging.FromContext(ctx).Debug("module input unresolved", "module", name, "input", key, "errors", diags.Errs())
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
	vars, err := getVariableDefaults(ctx, dir)
	if err != nil {
		// The module's inputs still resolve; its defaults don't.
		logging.FromContext(ctx).Warn("module variable defaults unavailable", "dir", dir, "error", err)
		vars = make(map[string]cty.Value)
	}
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
			logger.Debug("module output unresolved", "dir", dir, "output", o.Labels[0], "errors", diags.Errs())
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
			logging.FromContext(ctx).Warn("skipping unparsable terraform file", "file", f, "block", blockType, "error", diags.Error())
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
