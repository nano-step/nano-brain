package graph

type binding struct {
	target     string
	unresolved bool
}

type lexicalScope struct {
	parent   *lexicalScope
	function *lexicalScope
	bindings map[string]binding
}

func newLexicalScope(parent *lexicalScope, function bool) *lexicalScope {
	scope := &lexicalScope{parent: parent, bindings: make(map[string]binding)}
	if function || parent == nil {
		scope.function = scope
	} else {
		scope.function = parent.function
	}
	return scope
}

func (s *lexicalScope) childBlock() *lexicalScope {
	return newLexicalScope(s, false)
}

func (s *lexicalScope) childFunction() *lexicalScope {
	return newLexicalScope(s, true)
}

func (s *lexicalScope) declareLexical(name, target string) {
	s.bindings[name] = binding{target: target, unresolved: target == ""}
}

func (s *lexicalScope) declareVar(name string) {
	s.function.bindings[name] = binding{unresolved: true}
}

func (s *lexicalScope) declareFunction(name, target string) {
	s.function.bindings[name] = binding{target: target}
}

func (s *lexicalScope) declareClass(name, target string) {
	s.declareLexical(name, target)
}

func (s *lexicalScope) declareParameter(name string) {
	s.bindings[name] = binding{unresolved: true}
}

func (s *lexicalScope) lookup(name string) binding {
	for current := s; current != nil; current = current.parent {
		if value, ok := current.bindings[name]; ok {
			return value
		}
	}
	return binding{unresolved: true}
}
