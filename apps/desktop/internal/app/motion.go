package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's motion vocabulary. Every duration and spring lives here, so the
// whole window moves at one tempo and a change of mind is one edit.
//
// The rule behind the numbers: motion explains a change of state, it never
// performs. Nothing moves for decoration, nothing loops, and nothing takes
// longer than the eye needs to follow a card appearing.
var (
	// pageEnter brings a page in: it rises a little as it fades up, so a
	// navigation reads as arriving somewhere rather than as a redraw.
	// Measure before trusting the sign: a positive Enter.Y starts the element
	// above its place and slides it down, which reads as falling in. A page
	// should rise, so both offsets are negative.
	pageEnter = ui.Motion{Y: -10, Opacity: 1}

	// rowEnter is the same idea for a list that fills in: the connectivity
	// rows, stacked with a short delay each so they settle in a cascade.
	rowEnter = ui.Motion{Y: -6, Opacity: 1}

	// pageTransition is the timing of a page change. Short enough that a
	// page never feels slow to answer a click, long enough to be seen.
	pageTransition = ui.ElementTransition{
		Duration: 220 * time.Millisecond,
		Ease:     ui.EaseOut,
		Enter:    &pageEnter,
	}

	// rowTransition lets the probe rows arrive in order. The delay is applied
	// per row by rowDelay, not here: this one is the shared motion.
	rowTransition = ui.ElementTransition{
		Duration: 180 * time.Millisecond,
		Ease:     ui.EaseOut,
		Enter:    &rowEnter,
	}

	// heroSpring is how the status ring answers a change of state: a small
	// overshoot that reads as the app reacting, not as a bounce toy.
	heroSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.22}}

	// numberSpring is slower and softer: a rate that jumps to a new value
	// every second must not twitch.
	numberSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond}}
)

// rowDelayTime is how long row i waits before it arrives.
func rowDelayTime(i int) time.Duration {
	if i < 0 {
		i = 0
	}
	// Past a handful, extra delay only makes the list feel slow.
	if i > 7 {
		i = 7
	}
	return time.Duration(i) * 28 * time.Millisecond
}

// rowDelay is the spring a row enters with, delayed by its place in the list.
func rowDelay(i int) *ui.Animation {
	a := ui.Animation{Spring: ui.SpringOptions{Duration: 200 * time.Millisecond}}
	a.Delay = rowDelayTime(i)
	return &a
}
