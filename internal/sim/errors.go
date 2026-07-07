package sim

import "errors"

var (
	ErrInsufficientFuel  = errors.New("insufficient fuel")
	ErrInsufficientFunds = errors.New("insufficient credits")
	ErrHullBreached      = errors.New("hull breached — repair required")
	ErrNotDocked         = errors.New("must be docked at port")
	ErrInBelt            = errors.New("must be docked to use port services")
	ErrActiveRun         = errors.New("mining run in progress")
	ErrNoActiveRun       = errors.New("no active mining run")
	ErrInvalidWorld      = errors.New("invalid world")
	ErrInvalidAsteroid   = errors.New("invalid asteroid")
	ErrAlreadyFull       = errors.New("already at maximum")
	ErrMaxUpgrade        = errors.New("upgrade at max level")
	ErrNotEligible       = errors.New("not eligible")
)
