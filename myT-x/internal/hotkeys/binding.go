package hotkeys

// Modifier represents a Win32 hotkey modifier bitmask.
type Modifier uint32

// VKey represents a Win32 virtual-key code.
type VKey uint32

// Binding describes a parsed global hotkey.
// Construct only via ParseBinding to guarantee invariant consistency.
type Binding struct {
	modifiers  Modifier
	key        VKey
	normalized string
}

// Modifiers returns the modifier bitmask.
func (b Binding) Modifiers() Modifier { return b.modifiers }

// Key returns the virtual-key code.
func (b Binding) Key() VKey { return b.key }

// Normalized returns the canonical human-readable binding string.
func (b Binding) Normalized() string { return b.normalized }
