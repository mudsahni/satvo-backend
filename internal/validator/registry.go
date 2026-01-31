package validator

// Registry maps builtin_rule_key to Validator implementations.
type Registry struct {
	validators map[string]Validator
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{validators: make(map[string]Validator)}
}

// Register adds a validator to the registry.
func (r *Registry) Register(v Validator) {
	r.validators[v.RuleKey()] = v
}

// Get returns the validator for a given rule key, or nil if not found.
func (r *Registry) Get(key string) Validator {
	return r.validators[key]
}

// All returns all registered validators.
func (r *Registry) All() []Validator {
	out := make([]Validator, 0, len(r.validators))
	for _, v := range r.validators {
		out = append(out, v)
	}
	return out
}
