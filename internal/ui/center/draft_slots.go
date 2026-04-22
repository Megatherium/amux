package center

import (
	tea "charm.land/bubbletea/v2"
)

// setHarness sets the harness and populates model/agent options.
// Advances to SlotModel. Does NOT apply defaults.
func (d *Draft) setHarness(name string) {
	d.harness = name
	harnessCfg, ok := d.config.Assistants[name]
	if !ok {
		d.modelOptions = []string{"default"}
		d.agentOptions = []string{"default"}
	} else {
		d.modelOptions = stringOrDefault(harnessCfg.SupportedModels)
		d.agentOptions = stringOrDefault(harnessCfg.SupportedAgents)
	}
	d.model = ""
	d.agent = ""
	d.activeSlot = SlotModel
	d.resetFilter(d.modelOptions)
}

// stringOrDefault returns the list or ["default"] if empty.
func stringOrDefault(ss []string) []string {
	if len(ss) > 0 {
		out := make([]string, len(ss))
		copy(out, ss)
		return out
	}
	return []string{"default"}
}

// selectSlotOption selects the option at idx for the current slot and
// advances exactly one step. Returns false if idx is out of range.
func (d *Draft) selectSlotOption(idx int) bool {
	switch d.activeSlot {
	case SlotHarness:
		if idx >= len(d.harnessOptions) {
			return false
		}
		d.setHarness(d.harnessOptions[idx])
		return true

	case SlotModel:
		if idx >= len(d.modelOptions) {
			return false
		}
		d.model = d.modelOptions[idx]
		d.activeSlot = SlotAgent
		d.resetFilter(d.agentOptions)
		return true

	case SlotAgent:
		if idx >= len(d.agentOptions) {
			return false
		}
		d.agent = d.agentOptions[idx]
		d.activeSlot = SlotConfirm
		return true
	}
	return false
}

// applyDefaults advances through remaining slots when config defaults
// match available options. Stops at the first slot without a matching
// default, or at SlotConfirm when all defaults are resolved.
func (d *Draft) applyDefaults() {
	def := d.config.Defaults
	if def == nil {
		return
	}

	// Auto-fill model if a default is configured and matches an option.
	if d.activeSlot == SlotModel && def.Model != "" {
		for _, m := range d.modelOptions {
			if m == def.Model {
				d.model = m
				d.activeSlot = SlotAgent
				d.resetFilter(d.agentOptions)
				break
			}
		}
	}

	// Auto-fill agent if a default is configured and matches an option.
	if d.activeSlot == SlotAgent && def.Agent != "" {
		for _, a := range d.agentOptions {
			if a == def.Agent {
				d.agent = a
				d.activeSlot = SlotConfirm
				return
			}
		}
	}
}

// confirmSelection selects the option at idx and applies defaults.
// For SlotHarness, only the model default is applied (matching confirmHarness
// behavior). For SlotModel and SlotAgent, all remaining defaults cascade.
func (d *Draft) confirmSelection(idx int) (*Draft, tea.Cmd) {
	if !d.selectSlotOption(idx) {
		return d, nil
	}
	if d.activeSlot == SlotModel {
		// Harness was just selected — only auto-fill model, not agent.
		d.applyModelDefault()
	} else {
		d.applyDefaults()
	}
	return d, nil
}

// applyModelDefault fills the model from config defaults if a matching
// option exists. Does NOT cascade into agent auto-fill.
func (d *Draft) applyModelDefault() {
	def := d.config.Defaults
	if def == nil || def.Model == "" || d.activeSlot != SlotModel {
		return
	}
	for _, m := range d.modelOptions {
		if m == def.Model {
			d.model = m
			d.activeSlot = SlotAgent
			d.resetFilter(d.agentOptions)
			return
		}
	}
}

// confirmHarness sets the harness and auto-fills the model default.
// It does NOT auto-fill the agent — the user must explicitly reach the
// agent step before the agent default is applied.
func (d *Draft) confirmHarness(name string) {
	d.setHarness(name)
	d.applyModelDefault()
}
