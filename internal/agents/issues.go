package agents

import "sync"

// Issues is a small, concurrency-safe collector agents use to accumulate the
// problems they hit during a run. Each agent embeds its own collector and
// returns the list in its output; the pipeline aggregates them for the gate.
type Issues struct {
	mu   sync.Mutex
	list []AgentIssue
}

// Add records an issue from the given agent.
func (is *Issues) Add(agent, code, message, context string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.list = append(is.list, AgentIssue{
		Code:        code,
		SourceAgent: agent,
		Message:     message,
		Context:     context,
	})
}

// List returns a copy of the collected issues.
func (is *Issues) List() []AgentIssue {
	is.mu.Lock()
	defer is.mu.Unlock()
	out := make([]AgentIssue, len(is.list))
	copy(out, is.list)
	return out
}
