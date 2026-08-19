package main

import (
	"maps"
	"slices"

	"github.com/maruel/natural"
)

// TestStorage maps keys (package + test name) to all associated events.
type TestStorage map[Key]Events

// OrderedKeys returns sorted keys for stable presentation order.
func (ts TestStorage) OrderedKeys() []Key {
	var tks []Key
	for k := range ts {
		tks = append(tks, k)
	}
	slices.SortStableFunc(tks, func(a, b Key) int {
		if a.Package == b.Package && (a.Test == "" || b.Test == "") {
			if len(a.Test) != len(b.Test) {
				if len(a.Test) > len(b.Test) {
					return -1
				}
				return 1
			}
		}
		if natural.Less(a.String(), b.String()) {
			return -1
		}
		if natural.Less(b.String(), a.String()) {
			return 1
		}
		return 0
	})

	return tks
}

// Append adds an event into the storage.
func (ts TestStorage) Append(e Event) {
	if e.Package == "" && e.ImportPath != "" {
		e.Package = e.ImportPath
	}
	if e.Action == ActionBuildOutput {
		e.Action = ActionOutput
	}
	key := e.Key()
	events := ts[key]
	events = append(events, e)
	ts[key] = events
}

// Union merges multiple TestStorage instances.
func (ts TestStorage) Union(values ...TestStorage) TestStorage {
	tests := make(TestStorage, len(ts))
	maps.Copy(tests, ts)
	for _, v := range values {
		maps.Copy(tests, v)
	}
	return tests
}

// Filter returns a new TestStorage containing only entries that satisfy the predicate.
func (ts TestStorage) Filter(predicate func(k Key, events Events) bool) TestStorage {
	tests := make(TestStorage)
	for key, events := range ts {
		if predicate(key, events) {
			tests[key] = events
		}
	}
	return tests
}

// FilterPackageResults returns only test-level events (non-empty Test).
func (ts TestStorage) FilterPackageResults() TestStorage {
	return ts.Filter(func(k Key, _ Events) bool {
		return k.Test != ""
	})
}

// FindPackageResults returns only package-level events (empty Test).
func (ts TestStorage) FindPackageResults() TestStorage {
	return ts.Filter(func(k Key, _ Events) bool {
		return k.Test == ""
	})
}

// FilterKeys returns tests excluding those in the exclude map.
func (ts TestStorage) FilterKeys(exclude map[Key]bool) TestStorage {
	return ts.Filter(func(k Key, _ Events) bool {
		return !exclude[k]
	})
}

// FindPackageTests returns all events matching a package name.
func (ts TestStorage) FindPackageTests(name string) TestStorage {
	return ts.Filter(func(k Key, _ Events) bool {
		return k.Package == name
	})
}

// FindByAction returns tests containing at least one event matching the action.
func (ts TestStorage) FindByAction(action Action) TestStorage {
	return ts.Filter(func(_ Key, events Events) bool {
		for _, e := range events {
			if e.Action == action {
				return true
			}
			if action == ActionBuildFail && e.Action == ActionFail && e.FailedBuild != "" {
				return true
			}
		}
		return false
	})
}

// FilterAction returns tests that do NOT contain any of the specified actions.
func (ts TestStorage) FilterAction(actions ...Action) TestStorage {
	actionMatch := make(map[Action]bool, len(actions))
	for _, action := range actions {
		actionMatch[action] = true
	}
	return ts.Filter(func(_ Key, events Events) bool {
		for _, e := range events {
			if actionMatch[e.Action] {
				return false
			}
		}
		return true
	})
}

// FindByStatus returns tests matching a terminal status.
func (ts TestStorage) FindByStatus(status Status) TestStorage {
	return ts.Filter(func(_ Key, events Events) bool {
		return events.Status() == status
	})
}

// WithCoverage returns package-level tests that have coverage data.
func (ts TestStorage) WithCoverage() TestStorage {
	return ts.Filter(func(k Key, events Events) bool {
		return k.Test == "" && k.Package != "" && events.FindCoverage() != ""
	})
}

// FilterNotests returns tests excluding packages without any test files.
func (ts TestStorage) FilterNotests() TestStorage {
	return ts.Filter(func(_ Key, events Events) bool {
		return !events.IsPackageWithoutTest()
	})
}

// CountTests counts distinct tests and package build failures.
func (ts TestStorage) CountTests() int {
	count := 0
	for key, events := range ts {
		if key.Test != "" {
			count++
			continue
		}
		if events.Status() == StatusBuildFail {
			count++
		}
	}
	return count
}

// TestStats holds aggregated counts of test results.
type TestStats struct {
	Pass      int
	Fail      int
	BuildFail int
	None      int
	Skip      int
}

// Stats aggregates summary test metrics in a single pass.
func (ts TestStorage) Stats() TestStats {
	allPass := ts.FindByStatus(StatusPass)
	allFail := ts.FindByStatus(StatusFail)
	allBuildFail := ts.FindByStatus(StatusBuildFail)
	allSkip := ts.FindByStatus(StatusSkip)
	allNone := ts.FindByStatus(StatusNone)

	return TestStats{
		Pass:      allPass.CountTests(),
		Fail:      allFail.CountTests(),
		BuildFail: allBuildFail.CountTests(),
		None:      len(allNone),
		Skip:      allSkip.CountTests(),
	}
}
