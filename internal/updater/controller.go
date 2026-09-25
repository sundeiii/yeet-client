package updater

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

const DefaultCheckInterval = 2 * time.Hour

type CheckRequest struct {
	// Manual indicates that the check was requested by the user
	Manual bool
	// Force updates to the update candidate no matter what
	Force bool
	// Branch to check for updates on, can be left empty to use the default branch
	Branch *Branch
}

type CheckResult struct {
	Request CheckRequest
	Error   error
	Manual  bool

	CurrentVersion Version
	Branch         Branch
	Candidate      ReleaseCandidate
	CheckedAt      time.Time
}

type Controller struct {
	versionResolver func(Branch) (Version, error)
	branch          func() Branch
	interval        time.Duration

	requests       chan CheckRequest
	resultCallback func(CheckResult) (shouldRestart bool)

	automaticChecksEnabled func() bool
	check                  func(Version, Branch, bool) (ReleaseCandidate, error)

	isChecking atomic.Bool // true if a check is currently running or queued
}

func (controller *Controller) WithVersionResolver(resolver func(Branch) (Version, error)) *Controller {
	if resolver == nil {
		panic("version resolver cannot be nil")
	}
	controller.versionResolver = resolver
	return controller
}

func (controller *Controller) WithInterval(interval time.Duration) *Controller {
	if interval <= 0 {
		interval = DefaultCheckInterval
	}
	controller.interval = interval
	return controller
}

func (controller *Controller) WithBranch(branch func() Branch) *Controller {
	if branch == nil {
		panic("branch function cannot be nil")
	}
	controller.branch = branch
	return controller
}

func (controller *Controller) WithCallback(callback func(CheckResult) bool) *Controller {
	if callback == nil {
		panic("result callback cannot be nil")
	}
	controller.resultCallback = callback
	return controller
}

func (controller *Controller) WithAutomaticChecksEnabled(enabled func() bool) *Controller {
	if enabled == nil {
		panic("function cannot be nil")
	}
	controller.automaticChecksEnabled = enabled
	return controller
}

func NewController(versionResolver func(Branch) (Version, error), interval time.Duration) (*Controller, error) {
	if interval <= 0 {
		interval = DefaultCheckInterval
	}
	if versionResolver == nil {
		return nil, errors.New("version resolver cannot be nil")
	}
	return &Controller{
		versionResolver:        versionResolver,
		interval:               interval,
		requests:               make(chan CheckRequest, 1),
		branch:                 func() Branch { return BranchStable },
		resultCallback:         func(CheckResult) bool { return false },
		automaticChecksEnabled: func() bool { return false },
		check:                  Check,
	}, nil
}

// RequestCheck queues a check unless one is already queued or running.
func (controller *Controller) RequestCheck(request CheckRequest) bool {
	if !controller.isChecking.CompareAndSwap(false, true) {
		// A check is already queued or running, so we won't queue another one
		return false
	}

	select {
	case controller.requests <- request:
		return true
	default:
		// The request channel is full, so we can't queue another one
		controller.isChecking.Store(false)
		return false
	}
}

// Run processes requested checks at the specified interval.
func (controller *Controller) Run(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case request := <-controller.requests:
			// We received a request to check for updates
			// We stop the timer to avoid a race condition
			stopTimer(timer)
			if controller.performCheck(request) {
				return
			}
			// Restart the timer after the check is done
			timer.Reset(controller.interval)
		case <-timer.C:
			if !controller.isChecking.CompareAndSwap(false, true) {
				// A check is already queued or running, so we won't queue another one
				timer.Reset(controller.interval)
				continue
			}
			if controller.automaticChecksEnabled() {
				if controller.performCheck(CheckRequest{}) {
					return
				}
			} else {
				controller.isChecking.Store(false)
			}
			timer.Reset(controller.interval)
		}
	}
}

func (controller *Controller) performCheck(request CheckRequest) bool {
	defer controller.isChecking.Store(false)

	branch := controller.branch()
	if request.Branch != nil && branch != *request.Branch {
		// Switching branches should install always that branch
		request.Force = true
		branch = *request.Branch
	}
	currentVersion, err := controller.versionResolver(branch)

	var candidate ReleaseCandidate
	if err == nil {
		if currentVersion == nil {
			err = errors.New("update version resolver returned no version")
		} else {
			candidate, err = controller.check(currentVersion, branch, request.Force)
		}
	}

	result := CheckResult{
		Request:        request,
		Manual:         request.Manual,
		Branch:         branch,
		CurrentVersion: currentVersion,
		Candidate:      candidate,
		CheckedAt:      time.Now(),
		Error:          err,
	}
	// Callback will determine if we should restart the application after the update
	return controller.resultCallback != nil && controller.resultCallback(result)
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
