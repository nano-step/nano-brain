## Why

The Ruby class index resolves controller short names to wrong files. `TokensController` resolves to `app/models/tokens_controller.rb` instead of `app/controllers/api/v1/tokens_controller.rb`. This causes reconcile edges to point to wrong files, and since those wrong files have 0 outgoing calls, flows stop at 3 nodes (entry → handler → func). Cross-file call chains never expand.

## What Changes

- Fix `Lookup` in `ruby_class_index.go` to prefer full namespace match over short name fallback
- Add directory-based preference: `app/controllers/` preferred over `app/models/` when short names collide
- Reconcile edges will correctly target controller files → outgoing calls expand → flows reach 5+ nodes

## Impact

- **Code affected**: `internal/graph/ruby_class_index.go` — Lookup method fix
- **Risk**: Low — only changes lookup priority, no schema or API changes
- **Verification**: rails-app flows should show controller→service→model chains
