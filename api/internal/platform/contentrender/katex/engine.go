// Package katex renders bounded, untrusted TeX expressions to MathML with the
// exact KaTeX runtime vendored into the Judgify API.
package katex

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/dop251/goja"
)

const renderFunctionName = "__judgifyKaTeXRender"

const bootstrapSource = `
(function () {
	"use strict";
	if (typeof katex !== "object" || katex === null) {
		throw new Error("KaTeX runtime missing");
	}
	Object.defineProperty(globalThis, "` + renderFunctionName + `", {
		configurable: false,
		enumerable: false,
		writable: false,
		value: function (expression, displayMode) {
			try {
				return katex.renderToString(expression, {
					displayMode: displayMode === true,
					globalGroup: false,
					macros: {},
					maxExpand: 256,
					maxSize: 20,
					output: "mathml",
					strict: "error",
					throwOnError: true,
					trust: false
				});
			} catch (error) {
				if (error !== null && typeof error === "object" &&
					error.name === "ParseError") {
					return null;
				}
				throw error;
			}
		}
	});
})();
`

type runtimeSlot struct {
	runtime *goja.Runtime
	render  goja.Callable
}

// Engine is a concurrency-safe, hard-bounded pool of initialized KaTeX
// runtimes. A goja.Runtime is never shared by concurrent calls.
type Engine struct {
	programs *programs
	pool     chan *runtimeSlot
}

var defaultEngine = sync.OnceValues(func() (*Engine, error) {
	return New()
})

// Default returns the process-shared engine used by the authored-content
// pipeline. Sharing one bounded pool avoids multiplying initialized JavaScript
// heaps when callers construct multiple Renderer values.
func Default() (*Engine, error) {
	return defaultEngine()
}

// New authenticates and compiles the embedded KaTeX bundle, initializes every
// pooled runtime, and verifies the runtime-reported KaTeX version.
func New(options ...Option) (*Engine, error) {
	configuration := defaultConfig()
	for _, option := range options {
		if option == nil {
			return nil, ErrInvalidConfiguration
		}
		if err := option(&configuration); err != nil {
			return nil, ErrInvalidConfiguration
		}
	}

	compiled, err := sharedPrograms()
	if err != nil {
		return nil, err
	}
	engine := &Engine{
		programs: compiled,
		pool:     make(chan *runtimeSlot, configuration.poolSize),
	}
	for range configuration.poolSize {
		slot, slotErr := newRuntimeSlot(compiled)
		if slotErr != nil {
			return nil, slotErr
		}
		engine.pool <- slot
	}
	return engine, nil
}

// Render returns a newly owned MathML fragment. It is safe for concurrent use.
//
// Context cancellation is honored while waiting for a runtime and while
// JavaScript is executing. An interrupted runtime is discarded rather than
// cleared and reused, eliminating late-interrupt poisoning races.
func (engine *Engine) Render(
	ctx context.Context,
	expression []byte,
	displayMode bool,
) ([]byte, error) {
	if engine == nil || engine.programs == nil || engine.pool == nil {
		return nil, ErrInvalidEngine
	}
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateExpression(expression); err != nil {
		return nil, err
	}

	slot, err := engine.acquire(ctx)
	if err != nil {
		return nil, err
	}
	healthy := true
	defer func() {
		engine.release(slot, healthy)
	}()

	// Cancellation may race a ready pool token. Recheck it after acquisition
	// before installing an interrupt watcher.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	callDone := make(chan struct{})
	watcherDone := make(chan struct{})
	var interruptRequested atomic.Bool
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			interruptRequested.Store(true)
			slot.runtime.Interrupt(ctx.Err())
		case <-callDone:
		}
	}()

	value, callErr, panicked := callRenderer(slot, expression, displayMode)
	close(callDone)
	<-watcherDone

	// Joining the watcher before releasing the slot guarantees that no
	// goroutine can deliver a late interrupt to a reused runtime.
	if contextErr := ctx.Err(); contextErr != nil {
		if interruptRequested.Load() {
			healthy = false
		}
		return nil, contextErr
	}
	if interruptRequested.Load() {
		healthy = false
		return nil, ErrInternal
	}
	if panicked {
		healthy = false
		return nil, ErrInternal
	}
	if callErr != nil {
		var interrupted *goja.InterruptedError
		if errors.As(callErr, &interrupted) {
			healthy = false
			return nil, ErrInternal
		}
		healthy = false
		return nil, ErrInternal
	}
	if goja.IsNull(value) {
		return nil, ErrInvalidExpression
	}

	exported := value.Export()
	rendered, ok := exported.(string)
	if !ok || rendered == "" {
		healthy = false
		return nil, ErrInternal
	}
	if len(rendered) > MaximumOutputBytes {
		// The runtime has materialized an attacker-controlled oversized string.
		// Discard it so that heap cannot remain pinned in an idle pool slot.
		healthy = false
		return nil, ErrOutputTooLarge
	}
	if err := validateOutputResourceBounds([]byte(rendered)); err != nil {
		return nil, err
	}
	return []byte(rendered), nil
}

func (engine *Engine) acquire(ctx context.Context) (*runtimeSlot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case slot := <-engine.pool:
		if slot != nil {
			return slot, nil
		}
		replacement, err := newRuntimeSlot(engine.programs)
		if err != nil {
			// A nil entry preserves the semaphore token and allows a later call
			// to retry without silently shrinking the bounded pool.
			engine.pool <- nil
			return nil, ErrInternal
		}
		return replacement, nil
	}
}

func (engine *Engine) release(slot *runtimeSlot, healthy bool) {
	if !healthy || slot == nil {
		engine.pool <- nil
		return
	}
	engine.pool <- slot
}

func newRuntimeSlot(compiled *programs) (slot *runtimeSlot, err error) {
	defer func() {
		if recover() != nil {
			slot = nil
			err = ErrInternal
		}
	}()
	if compiled == nil || compiled.katex == nil || compiled.bootstrap == nil {
		return nil, ErrInternal
	}

	runtime := goja.New()
	if _, runErr := runtime.RunProgram(compiled.katex); runErr != nil {
		return nil, ErrInternal
	}
	katexValue := runtime.Get("katex")
	if katexValue == nil || goja.IsNull(katexValue) || goja.IsUndefined(katexValue) {
		return nil, ErrInternal
	}
	versionValue := katexValue.ToObject(runtime).Get("version")
	version, ok := versionValue.Export().(string)
	if !ok {
		return nil, ErrInternal
	}
	if version != Version {
		return nil, ErrVersionMismatch
	}

	if _, runErr := runtime.RunProgram(compiled.bootstrap); runErr != nil {
		return nil, ErrInternal
	}
	render, ok := goja.AssertFunction(runtime.Get(renderFunctionName))
	if !ok {
		return nil, ErrInternal
	}
	return &runtimeSlot{
		runtime: runtime,
		render:  render,
	}, nil
}

func callRenderer(
	slot *runtimeSlot,
	expression []byte,
	displayMode bool,
) (value goja.Value, err error, panicked bool) {
	defer func() {
		if recover() != nil {
			value = nil
			err = nil
			panicked = true
		}
	}()
	value, err = slot.render(
		goja.Undefined(),
		slot.runtime.ToValue(string(expression)),
		slot.runtime.ToValue(displayMode),
	)
	return value, err, false
}
