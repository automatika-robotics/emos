# Internal-State Events

Most events fire on **topic data** — *"battery dropped below 15%"*, *"a person was detected"*, *"the controller reported failure"*. These are clean and declarative, but they share an assumption: the trigger condition can be expressed as a predicate over messages flowing through the system as a published topic.

Sometimes that's not enough. The thing you want to react to is **internal state**:

- A hardware monitor that is not being published as a topic.
- A compound predicate over multiple disparate signals.
- A hysteresis flag that's *sticky* across cycles.

For these cases, EMOS supports **callable-based events**: an `Event` constructed from any Python callable that returns `bool`, polled at a configurable rate. The callable holds whatever state it needs — a closure variable, a class attribute, a global counter — and the Monitor polls it at `check_rate` Hz. When it returns `True`, registered Actions fire just like for any other event.

```{seealso}
For the conceptual reference, see [Events & Actions → Events from Internal State](../../concepts/events-and-actions.md#events-from-internal-state). For visualising live events in the Web UI, see [Visualizing Events](visualizing-events.md).
```

---

## The Mechanic

An `Event` accepts three kinds of `event_condition`:

1. A `Topic` — fires on any new message.
2. A `Condition` expression — fires when a predicate over topic attributes becomes true.
3. **A `Callable` returning `bool`** — fires when polling the callable returns true.

The third form is what we use here:

```python
from ros_sugar.event import Event

def is_low_power() -> bool:  # must be type-annotated as bool
    ...

event_low_power = Event(is_low_power, check_rate=1.0)  # polled every 1 second
```

Two rules from the `Event` constructor:

- The callable's return type annotation **must be `bool`**. The constructor inspects it and raises `TypeError` otherwise.
- The callable **cannot be a `@component_action`** method bound to a managed component's lifecycle. Use a plain function or a regular instance method instead.

`check_rate` is in **Hz**. If omitted, the event polls at the central Monitor's loop rate. For most real-world predicates, anywhere from 0.5–5 Hz is sensible — fast enough to catch transitions, slow enough to be cheap.

---

## Recipe: Idle Detection

A robot that listens for voice commands should react when there's been no input for a while — dim its display, switch to a lighter local model, or just go quiet. *"Idleness"* isn't a value on any topic; it's a derived predicate over time and the most recent input. Perfect fit for a callable-based event.

### Step 1: A stateful predicate

Hold the *last interaction* timestamp in a tiny helper class and expose a typed `is_idle` method:

```python
import time


class IdleTracker:
    """Tracks how long it has been since the last user interaction."""

    def __init__(self, threshold_seconds: float = 60.0) -> None:
        self.threshold_seconds = threshold_seconds
        self._last_seen = time.time()

    def touch(self) -> None:
        """Mark 'just had user interaction' -- call this from your input pipeline."""
        self._last_seen = time.time()

    def is_idle(self) -> bool:
        """True when we haven't seen interaction for ``threshold_seconds``."""
        return (time.time() - self._last_seen) > self.threshold_seconds


idle = IdleTracker(threshold_seconds=60.0)
```

Two methods — `touch()` to update state, `is_idle()` to read it. The `-> bool` annotation on `is_idle` is mandatory; the `Event` constructor inspects it.

### Step 2: Wire the state update

Whenever the Speech-to-Text component publishes a transcribed query, we want to call `idle.touch()`. EMOS lets us hook a small pre-processor on the publishing side of any topic:

```python
from typing import Optional


def touch_on_speech(text: str) -> Optional[str]:
    idle.touch()
    return text  # pass through unchanged


speech_to_text.add_publisher_preprocessor(query_topic, touch_on_speech)
```

The pre-processor runs in-process before publication. It updates `idle._last_seen` and passes the message through untouched. Any topic update mechanism would work — a callback, a periodic component, a subscriber elsewhere. The point is that the *state* lives in `idle`, not in any topic.

### Step 3: Build the Event and an Action

```python
from ros_sugar.actions import log
from ros_sugar.event import Event

event_idle = Event(idle.is_idle, check_rate=0.5)  # poll every 2 seconds

events_actions = {
    event_idle: [
        log(msg="No interaction for 60s -- switching to low-power mode"),
        # ...switch to a smaller model, dim a display, etc.
    ],
}
```

`check_rate=0.5` means Mermaid checks `idle.is_idle()` every 2 seconds. As long as `touch_on_speech` keeps firing, `is_idle()` returns False; the moment 60 seconds elapse without a query, the next poll returns True and the Action fires.

### Step 4: Hand the events_actions dict to the Launcher

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.add_pkg(
    components=[speech_to_text, vlm, text_to_speech],
    events_actions=events_actions,
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.bringup()
```

The Monitor (or `Cortex`, if it's the recipe's monitor) takes ownership of the polling loop. From this point on, every 2 seconds the Monitor calls `idle.is_idle()` and fires the registered Actions on rising-edge transitions.

---

## When to reach for this pattern

Use a callable-based event when:

- The condition is **internal state** that has no business being a topic (counters, timers, sticky flags, last-seen timestamps).
- The condition is a **compound predicate** over multiple sources that's easier to express as an arbitrary calculation in Python rather than as a chain of topic conditions.
- You want **encapsulation** — keep the state inside the recipe rather than smearing it across publisher topics just to satisfy the trigger.

Use a topic-based event when the condition genuinely is *"some message arrived"* or *"this attribute crossed a threshold"*. Topic events are cheaper (push, no polling) and more declarative.

---

## Visualisation

Callable-based events show up on the [System Graph](../../concepts/web-ui.md) too: the source component (or the recipe-level dispatcher) is the upstream node, and the registered Actions are the downstream nodes. The detail card lists `check_rate` and the current trigger count, so you can confirm at runtime that the predicate is actually flipping when you expect it to. See [Visualizing Events](visualizing-events.md) for the tour.
