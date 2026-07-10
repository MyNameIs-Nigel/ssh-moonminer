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
	ErrNotScanned        = errors.New("asteroid not scanned yet")
	ErrAlreadyScanned    = errors.New("asteroid already scanned")
	ErrScanInProgress    = errors.New("scan already in progress")
	ErrInvalidRunPhase   = errors.New("action not valid in current run phase")
	ErrOutOfRange        = errors.New("asteroid is beyond scanner range")

	// Fleet/shipyard errors (gameplay/05-fleet-ships-and-shipyard-economy.md).
	ErrInvalidShip      = errors.New("unknown ship model")
	ErrAlreadyOwned     = errors.New("ship already owned")
	ErrNotOwned         = errors.New("ship not owned")
	ErrItemLocked       = errors.New("item is locked")
	ErrInvalidSlotItem  = errors.New("item does not fit this slot")
	ErrInvalidSlotIndex = errors.New("invalid slot index")
	ErrPowerExceeded    = errors.New("insufficient power capacity")
)
