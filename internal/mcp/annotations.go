package mcp

import sdk "github.com/modelcontextprotocol/go-sdk/mcp"

// Tool annotations tell MCP clients how a tool behaves, so they can
// auto-approve reads and ask the operator before destructive calls.
// They are hints: the confirm gate is what actually guards mutations.
// The spec defaults destructiveHint to true, so every mutating tool sets
// it explicitly.

// readOnly annotates a tool with no side effects.
func readOnly() *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{ReadOnlyHint: true}
}

// idempotent annotates a mutating tool that is safe to repeat: a second
// call with the same input changes nothing more.
func idempotent() *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true}
}

// disruptive annotates a mutating tool that is safe to repeat but not
// only additive: it evicts or replaces what is there (a drain evicts
// pods and restarts endpoints; set_tenant overwrites a name and note).
// The spec reserves destructiveHint false for additive updates.
func disruptive() *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true}
}

// recoverable annotates a mutating tool whose effect can be retried or
// undone, but which acts again on every call.
func recoverable() *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{DestructiveHint: new(false)}
}

// destructive annotates a mutating tool that irreversibly destroys state.
func destructive() *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{DestructiveHint: new(true)}
}
