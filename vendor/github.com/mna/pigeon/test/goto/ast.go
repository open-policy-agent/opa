package asmgoto

import "fmt"

// Instruction is an assembler instruction
type Instruction interface {
	// Assemble returns an assembler instruction as string representation
	Assemble() string
}

// Noop is the no-operation assembler instruction
type Noop struct{}

// Assemble returns an assembler instruction as string representation
func (n Noop) Assemble() string {
	return "noop"
}

// Jump is the jump assembler instruction
type Jump struct {
	RelAddr int
}

// Assemble returns an assembler instruction as string representation
func (j Jump) Assemble() string {
	return fmt.Sprintf("jump %d", j.RelAddr)
}

type labelLookup map[string]label

type label struct {
	line  int
	jumps []unresolvedJump
}

type unresolvedJump struct {
	line int
	jump *Jump
}

func addLabel(c *current, name string) {
	ll := c.globalStore["labelLookup"].(labelLookup)
	l, ok := ll[name]
	if !ok {
		// Label not seen yet, add to labelLookup
		ll[name] = label{
			line:  c.pos.line,
			jumps: []unresolvedJump{},
		}
	} else {
		// Label already seen
		if l.line != -1 {
			panic(fmt.Sprintf("label '%s' already defined on line %d", name, l.line))
		}
		// Update position for later usage of Label
		l.line = c.pos.line
		// Update all already known jumps to this Label with the correct relative jump distance
		for _, unresjump := range l.jumps {
			unresjump.jump.RelAddr = l.line - unresjump.line
		}
		l.jumps = []unresolvedJump{}
		ll[name] = l
	}
}

func addJump(c *current, name string) *Jump {
	ll := c.globalStore["labelLookup"].(labelLookup)
	l, ok := ll[name]
	j := Jump{}
	if !ok {
		// Label not seen yet, create Label with invalid line = -1, add Jump to unresolvedJump
		ll[name] = label{
			line: -1,
			jumps: []unresolvedJump{
				{
					line: c.pos.line,
					jump: &j,
				},
			},
		}
	} else {
		if l.line == -1 {
			// Label already seen as target of an other Jump, add Jump to unresolvedJump
			l.jumps = append(l.jumps, unresolvedJump{line: c.pos.line, jump: &j})
			ll[name] = l
		} else {
			// Label already seen, calculate correct relative jump distance
			j.RelAddr = l.line - c.pos.line
		}
	}
	return &j
}

func labelCheck(c *current) (bool, error) {
	ll := c.globalStore["labelLookup"].(labelLookup)
	// Iterate through all Label, there must be no unresolved jumps
	for name, l := range ll {
		if len(l.jumps) > 0 {
			return false, fmt.Errorf("jump to undefined label '%s' found", name)
		}
	}
	return true, nil
}
