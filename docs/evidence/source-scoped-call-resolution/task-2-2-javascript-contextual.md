# Task 2.2 durable summary: JavaScript contextual extraction (complete)

The JavaScript contextual route now supplies parser-derived imports, lexical
bindings, class methods, and call shapes to the shared per-extraction resolver.
It emits canonical targets only for direct named/default/namespace/class and
local supported forms; dynamic/computed, shadowed, external, missing, and
unresolved forms emit target-only `<unresolved>`.

Declaration containment now includes `Class.method`, `default`, and
`default.method` identities matching contextual call targets. Legacy plain
`ExtractEdges` continues to expose bare call targets. The AST import walker
also handles named imports whose exported name is `from`, avoiding textual
clause-splitting ambiguity.

Focused JS/race and legacy controls passed, including the `from` import parser
regression. TypeScript contextual extraction and reader controls are covered
by the corresponding completed task evidence.

Adversarial verification added lexical guards for member receivers and a
JavaScript `catch_clause` parameter scope. Anonymous default classes use the
conservative unresolved policy for method calls until an emitted identity can
be proven.
