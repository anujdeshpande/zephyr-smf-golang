# zephyr-smf-golang

A Go port of the [Zephyr RTOS](https://zephyrproject.org) State Machine
Framework (SMF): flat and hierarchical state machines with entry, run and exit
actions, parent states, initial transitions and instrumentation hooks.

**Ported from Zephyr [v4.4.2](https://github.com/zephyrproject-rtos/zephyr/tree/v4.4.2),
the latest Zephyr release when this port was made (SMF API version 0.2.0).**
It follows `lib/smf/smf.c` line for line, and it passes Zephyr's complete SMF
test suite, `tests/lib/smf`, ported to Go. The behaviour is the same, so a state
machine designed for Zephyr SMF runs the same way here.

Standard library only. Go 1.22 or later.

```sh
go get github.com/anujdeshpande/zephyr-smf-golang@v0.1.0
```

```go
import "github.com/anujdeshpande/zephyr-smf-golang/smf"
```

Documentation: [pkg.go.dev](https://pkg.go.dev/github.com/anujdeshpande/zephyr-smf-golang/smf).

## Example

```go
const (
	parent = iota
	s0
	s1
)

// The user defined object embeds the context, as struct smf_ctx is the
// first member of the object in C.
type object struct {
	smf.Ctx[*object]
	count int
}

var states []smf.State[*object]

func init() {
	states = make([]smf.State[*object], 3)
	states[parent] = smf.State[*object]{
		Entry: func(o *object) { fmt.Println("parent entry") },
		Exit:  func(o *object) { fmt.Println("parent exit") },
	}
	states[s0] = smf.State[*object]{
		Run: func(o *object) smf.Result {
			o.SetState(&states[s1])
			return smf.Handled
		},
		Parent: &states[parent],
	}
	states[s1] = smf.State[*object]{
		Run: func(o *object) smf.Result {
			if o.count++; o.count == 3 {
				o.SetTerminate(1)
			}
			return smf.Propagate
		},
		Parent: &states[parent],
	}
}

func main() {
	obj := &object{}
	obj.SetInitial(obj, &states[s0])
	for obj.RunState() == 0 {
	}
}
```

The state table is filled in by `init` because the actions refer to it; a Go
variable initializer may not refer to itself.

## From C to Go

| Zephyr C | Go |
|---|---|
| `struct smf_ctx` as the first member of the object | `smf.Ctx[*Object]` embedded in `Object` |
| `struct smf_state`, `SMF_CREATE_STATE(entry, run, exit, parent, initial)` | `smf.State[*Object]{Entry, Run, Exit, Parent, Initial}` |
| `enum smf_state_result`: `SMF_EVENT_HANDLED`, `SMF_EVENT_PROPAGATE` | `smf.Result`: `smf.Handled`, `smf.Propagate` |
| `smf_set_initial(SMF_CTX(&obj), &states[S0])` | `obj.SetInitial(obj, &states[s0])` |
| `smf_set_state(SMF_CTX(obj), &states[S1])` | `obj.SetState(&states[s1])` |
| `smf_set_terminate(SMF_CTX(obj), val)` | `obj.SetTerminate(val)` |
| `smf_run_state(SMF_CTX(&obj))` | `obj.RunState()` |
| `smf_get_current_leaf_state(ctx)` | `obj.Current()` |
| `smf_get_current_executing_state(ctx)` | `obj.Executing()` |
| `ctx->previous` | `obj.Previous()` |
| `struct smf_hooks`, `smf_set_hooks(ctx, &hooks)` | `smf.Hooks[*Object]`, `obj.SetHooks(&hooks)` |
| `enum smf_action_type`: `SMF_ACTION_ENTRY`, `_RUN`, `_EXIT` | `smf.Action`: `smf.ActionEntry`, `ActionRun`, `ActionExit` |
| `SMF_ERR_NULL_TRANSITION`, `SMF_ERR_TRANSITION_IN_EXIT` | `smf.ErrNilTransition`, `smf.ErrTransitionInExit` |

Differences from the C version:

- **No Kconfig.** `CONFIG_SMF_ANCESTOR_SUPPORT`, `CONFIG_SMF_INITIAL_TRANSITION`
  and `CONFIG_SMF_INSTRUMENTATION` are always on. Each costs nothing when unused,
  and a state without a parent behaves exactly as in a flat Zephyr state machine.
  Zephyr's flat test suite passes against this single build.
- **Typed objects.** Actions take the user object as its own type, `func(*Object)`,
  instead of `void *`, so there is no cast and no "must be first" rule.
  `SetInitial` takes the object that actions receive.
- **No logging.** Where Zephyr logs `LOG_ERR` for an invalid `smf_set_state`,
  this port only reports the error to `Hooks.OnError`. A library should not
  write to the application's log.
- **Terminate value** is an `int` rather than `int32_t`.

## Tests

Every test in Zephyr's `tests/lib/smf` is ported in `smf/`, with its name,
comments, state tables and expected action and transition traces kept:

| Zephyr | Go |
|---|---|
| `test_lib_flat_smf.c` (4 tests) | `smf/flat_test.go` |
| `test_lib_hierarchical_smf.c` (7 tests) | `smf/hierarchical_test.go` |
| `test_lib_hierarchical_5_ancestor_smf.c` (1 test) | `smf/hierarchical_5_ancestor_test.go` |
| `test_lib_self_transition_smf.c` (7 tests) | `smf/self_transition_test.go` |
| `test_instrumentation_common.h` | `smf/instrumentation_test.go` |

`smf/errors_test.go` adds tests the Zephyr suite does not have, for the two
errors and `Previous` and `Executing`. `smf/example_test.go` holds the
examples from Zephyr's SMF documentation as Go examples.

```sh
go test -race ./...
```

## Samples

Both Zephyr SMF samples are ported in `examples/`:

- **`examples/hsm_psicc2`** (`samples/subsys/smf/hsm_psicc2`): the statechart
  in Figure 2.11 of *Practical UML Statecharts in C/C++*, 2nd Edition, by Miro
  Samek, which has every transition topology up to four levels of nesting.
  It logs every action. Commands come from standard input instead of the Zephyr
  shell:

  ```sh
  go run ./examples/hsm_psicc2
  event g
  event a
  terminate
  ```

- **`examples/smf_calculator`** (`samples/subsys/smf/smf_calculator`): a desk
  calculator built on the three-level statechart in Fig 2.18 of the same book.
  Keys come from standard input; the Zephyr sample's LVGL touchscreen interface
  is not ported.

  ```sh
  echo '12+30=' | go run ./examples/smf_calculator
  ```

Each sample has tests that check its behaviour against the statechart.

## Updating to a newer Zephyr

The port tracks a single Zephyr release. To move to a newer one, diff
`lib/smf/smf.c`, `include/zephyr/smf.h`, `tests/lib/smf` and
`samples/subsys/smf` between the two Zephyr tags, apply the changes here, and
update the version in this README and in the package documentation.

## License

Apache-2.0, the license of the Zephyr code this is ported from. See
[LICENSE](LICENSE) and [NOTICE](NOTICE). Each file keeps the copyright lines of
the Zephyr file it was ported from.
