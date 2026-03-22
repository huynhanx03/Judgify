---
name: test-common
description: Comprehensive testing skill for Common/shared packages. Ensures all reusable components are thoroughly tested with team-wide standards. Use for testing data structures, utilities, pools, and shared libraries.
---

# Test Common Skill

This skill provides **comprehensive testing standards** for Common/shared packages.
Since these packages are used by the entire team, tests must cover **ALL possible cases**.

---

## Quality Standards

| Metric | Requirement |
|--------|-------------|
| **Line Coverage** | Minimum 50% (per project rules) |
| **All Public APIs** | Must have tests |
| **Edge Cases** | Must be covered |
| **Panic Scenarios** | Must be tested |
| **Interface Compliance** | Must be verified |

---

## Workflow (Strict 4-Step Process)

### Step 1: Business Flow Design

**MANDATORY: Analyze EVERY public method in the file.**

1. List ALL exported functions/methods
2. For EACH method, identify:
   - Normal usage (happy path)
   - Error conditions
   - Edge cases
   - Panic conditions

**Output Format:**
```markdown
## Business Flows: [package/file]

### Method Inventory
| Method | Parameters | Returns | Notes |
|--------|------------|---------|-------|
| New(cap int) | int | *Buffer | Constructor |
| Write(p []byte) | []byte | (int, error) | io.Writer |
| ... | ... | ... | ... |

### Actors
- [List of actors]

### Flows per Method
For each method:
- Happy path
- Error cases
- Edge cases
```

**STOP and wait for user approval.**

---

### Step 2: Test Plan Design

**Define testing strategy:**

1. **Method Coverage Matrix**
   - List EVERY public method
   - Assign test count per method (minimum 3 per method)
   
2. **Scope**
   - In-scope: All public methods, exposed interfaces
   - Out-of-scope: Private methods, external dependencies

3. **Test Types**
   - Unit tests (isolated function testing)
   - Integration tests (multi-method workflows)
   - Fuzz tests (if applicable)

4. **Interface Compliance**
   - List all interfaces the type should implement

**Output Format:**
```markdown
### Method Coverage Matrix
| Method | Min Tests | Categories |
|--------|-----------|------------|
| New | 4 | Constructor, Boundary |
| Write | 5 | Input, State, Error |
| Read | 5 | Input, State, Error |
| ... | ... | ... |
| **Total** | **40+** | |
```

**STOP and wait for user approval.**

---

### Step 3: Test Case Design

**MANDATORY: Create tests for EVERY method from the inventory.**

For EACH method, apply these test categories:

#### Per-Method Test Matrix

| Category | Tests per Method | Description |
|----------|------------------|-------------|
| Happy Path | 1-2 | Normal valid usage |
| Nil/Empty Input | 1-2 | nil, empty slice, "" |
| Boundary | 2-3 | min, max, overflow |
| State | 1-2 | empty, full, partial |
| Error/Panic | 1-2 | Expected failures |
| **Minimum** | **5-10** | Per method |

#### Test Categories Reference

**Category 1: Constructor Tests (P0)**
| Class | Description |
|-------|-------------|
| Valid Creation | Normal instantiation |
| Zero/Default | Zero or default values |
| Negative | Negative values |
| Nil Config | Nil configuration |

**Category 2: Input Validation (P0)**
| Class | Description |
|-------|-------------|
| Nil Input | nil slice/pointer |
| Empty Input | empty slice/string |
| Zero Value | 0 for numeric |
| Negative | -1 where invalid |
| Overflow | math.MaxInt64 |

**Category 3: Boundary Tests (P0)**
| Class | Description |
|-------|-------------|
| Min Value | Minimum valid |
| Min - 1 | Below minimum |
| Max Value | Maximum valid |
| Max + 1 | Above maximum |

**Category 4: State Tests (P0)**
| Class | Description |
|-------|-------------|
| Empty State | Operations on empty |
| Full State | Operations on full |
| Partial State | Mid-state |
| After Reset | Post-reset |
| After Release | Post-release |

**Category 5: Sequence Tests (P1)**
| Class | Description |
|-------|-------------|
| Normal Sequence | Expected order |
| Repeated | Same op multiple times |
| Interleaved | Alternating operations |

**Category 6: Error Handling (P1)**
| Class | Description |
|-------|-------------|
| Expected Errors | Documented errors |
| Panic Conditions | Expected panics |
| Panic Recovery | defer recover() |

**Category 7: Interface Compliance (P1)**
| Class | Description |
|-------|-------------|
| io.Reader | Read interface |
| io.Writer | Write interface |
| io.WriterTo | WriteTo interface |
| io.ReaderFrom | ReadFrom interface |

**Output Format:**
```markdown
### Test Cases by Method

#### Method: New()
| ID | Category | Input | Expected |
|----|----------|-------|----------|
| TC-New-1 | Happy | New(100) | valid buffer |
| TC-New-2 | Boundary | New(0) | uses default |
| TC-New-3 | Boundary | New(-1) | uses default |
| TC-New-4 | Boundary | New(MAX) | valid buffer |

#### Method: Write()
| ID | Category | Input | Expected |
|----|----------|-------|----------|
| TC-Write-1 | Happy | Write(data) | n=len, nil |
| TC-Write-2 | Nil | Write(nil) | n=0, nil |
| TC-Write-3 | Empty | Write([]byte{}) | n=0, nil |
| TC-Write-4 | Large | Write(1MB) | auto-grow |
| TC-Write-5 | State | Write after Reset | works |

... (repeat for ALL methods)
```

**Verification Checklist:**
- [ ] Every public method has tests
- [ ] Every method has ≥ 5 test cases
- [ ] All panic conditions covered
- [ ] All error returns covered
- [ ] All interfaces verified

**STOP and wait for user approval.**

---

### Step 4: Test Code Implementation

**Follow these Go testing standards:**

#### File Structure
```
package_test.go
├── // Interface Compliance (compile-time)
│   var _ io.Writer = (*Type)(nil)
│
├── // Constructor Tests
│   func TestNew(t *testing.T)
│   func TestNew_Defaults(t *testing.T)
│
├── // Method Tests (ONE test function per method)
│   func TestWrite(t *testing.T) - all Write scenarios
│   func TestRead(t *testing.T) - all Read scenarios
│
├── // Sequence Tests
│   func TestWorkflow_NormalSequence(t *testing.T)
│
├── // Panic Tests
│   func TestPanic_NilData(t *testing.T)
```

#### Code Patterns

**Table-Driven Tests (PREFERRED):**
```go
func TestMethod(t *testing.T) {
    tests := []struct {
        name    string
        input   int
        want    int
        wantErr bool
    }{
        {"valid", 100, 100, false},
        {"zero", 0, defaultSize, false},
        {"negative", -1, 0, true},
        // Add ALL cases from test case design
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Method(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

**Panic Tests:**
```go
func TestPanic_Scenario(t *testing.T) {
    defer func() {
        if r := recover(); r == nil {
            t.Error("expected panic")
        }
    }()
    // Code that should panic
}
```

**Interface Compliance:**
```go
var _ io.Writer = (*Buffer)(nil)
var _ io.WriterTo = (*Buffer)(nil)
var _ io.ReaderFrom = (*Buffer)(nil)
```

---

## Rules (Non-negotiable)

1. **Never skip steps** - Each step builds on the previous
2. **Never assume approval** - Always wait for user confirmation
3. **Never write code early** - Code comes only in Step 4
4. **EVERY public method must have tests** - No exceptions
5. **Minimum 5 tests per method** - More for complex methods
6. **Test all errors** - Every error path must be tested
7. **Test all panics** - Every panic condition must be verified
8. **50% minimum coverage** - Per project requirements

---

## Priority Definitions

| Priority | Description | Must Pass? |
|----------|-------------|------------|
| **P0** | Critical functionality | YES - Blocks release |
| **P1** | Important scenarios | YES - Should fix |
| **P2** | Edge cases | NO - Nice to have |

---

## Approval Prompt

After each step, ask:

> "Please review the above and confirm if I should proceed to the next step."
