package sim

import (
	"cmp"
	"math"
	"sort"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// Track identifies one of a ship's five stat tracks. See
// docs/gameplay/05-fleet-ships-and-shipyard-economy.md.
type Track int

const (
	TrackThrusters Track = iota
	TrackHull
	TrackFuelEff
	TrackPowerGen
	TrackScanner
)

// TrackCount is the number of ship stat tracks.
const TrackCount = 5

// MaxGrade is the top grade (S) any track or slot device can reach.
const MaxGrade = 5

var gradeLetters = [...]string{"E", "D", "C", "B", "A", "S"}

// GradeLetter renders a 0..5 grade as its E..S letter.
func GradeLetter(grade int) string {
	if grade < 0 {
		grade = 0
	}
	if grade > MaxGrade {
		grade = MaxGrade
	}
	return gradeLetters[grade]
}

// TrackName returns the display name for a ship stat track.
func TrackName(t Track) string {
	switch t {
	case TrackThrusters:
		return "THRUSTERS"
	case TrackHull:
		return "HULL"
	case TrackFuelEff:
		return "FUEL EFFICIENCY"
	case TrackPowerGen:
		return "POWER GENERATOR"
	case TrackScanner:
		return "SCANNER"
	}
	return ""
}

// ActiveShip returns the currently flown ship instance, or nil if the
// hangar/active pointer is somehow unresolved (should not happen outside
// mid-migration — DecodeState always repairs this).
func ActiveShip(s *State) *ShipInstance {
	if s.Ships == nil {
		return nil
	}
	return s.Ships[s.ActiveShipID]
}

// OwnsShip reports whether the pilot currently owns modelID.
func OwnsShip(s *State, modelID string) bool {
	if s.Ships == nil {
		return false
	}
	_, ok := s.Ships[modelID]
	return ok
}

// trackGrade returns a ship instance's current grade for track.
func trackGrade(inst *ShipInstance, t Track) int {
	switch t {
	case TrackThrusters:
		return inst.Grades.Thrusters
	case TrackHull:
		return inst.Grades.Hull
	case TrackFuelEff:
		return inst.Grades.FuelEff
	case TrackPowerGen:
		return inst.Grades.PowerGen
	case TrackScanner:
		return inst.Grades.Scanner
	}
	return 0
}

func setTrackGrade(inst *ShipInstance, t Track, grade int) {
	switch t {
	case TrackThrusters:
		inst.Grades.Thrusters = grade
	case TrackHull:
		inst.Grades.Hull = grade
	case TrackFuelEff:
		inst.Grades.FuelEff = grade
	case TrackPowerGen:
		inst.Grades.PowerGen = grade
	case TrackScanner:
		inst.Grades.Scanner = grade
	}
}

// trackCap returns model's cap grade for track.
func trackCap(model *content.ShipModel, t Track) int {
	switch t {
	case TrackThrusters:
		return model.ThrustersCap
	case TrackHull:
		return model.HullCap
	case TrackFuelEff:
		return model.FuelEffCap
	case TrackPowerGen:
		return model.PowerGenCap
	case TrackScanner:
		return model.ScannerCap
	}
	return 0
}

func trackStart(model *content.ShipModel, t Track) int {
	switch t {
	case TrackThrusters:
		return model.ThrustersStart
	case TrackHull:
		return model.HullStart
	case TrackFuelEff:
		return model.FuelEffStart
	case TrackPowerGen:
		return model.PowerGenStart
	case TrackScanner:
		return model.ScannerStart
	}
	return 0
}

func trackBasePrice(c *content.Content, t Track) int {
	switch t {
	case TrackThrusters:
		return c.Fleet.ThrustersBasePrice
	case TrackHull:
		return c.Fleet.HullBasePrice
	case TrackFuelEff:
		return c.Fleet.FuelEffBasePrice
	case TrackPowerGen:
		return c.Fleet.PowerGenBasePrice
	case TrackScanner:
		return c.Fleet.ScannerBasePrice
	}
	return 0
}

// TrackCap returns model's cap grade for track — the highest grade that
// hull design can ever reach.
func TrackCap(model *content.ShipModel, t Track) int {
	return trackCap(model, t)
}

// TrackGrade returns shipID's current grade for track, or 0 if not owned.
func TrackGrade(s *State, shipID string, t Track) int {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	return trackGrade(inst, t)
}

// TrackPrice returns the credit cost to buy shipID's next grade of track.
// gameplay/05: price(track, ship, grade) = trackBaseUnit * ship.TierMul *
// GradePriceCurve^grade, charged for the grade being purchased (the ship's
// *current* grade).
func TrackPrice(c *content.Content, model *content.ShipModel, t Track, grade int) int {
	base := trackBasePrice(c, t)
	return int(math.Round(float64(base) * model.TierMul * math.Pow(c.Fleet.GradePriceCurve, float64(grade))))
}

// BuyShipTrack buys the next grade of track for shipID, which must be owned
// and docked, and below its model's cap.
func BuyShipTrack(s *State, c *content.Content, shipID string, t Track) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	inst := s.Ships[shipID]
	if inst == nil {
		return ErrInvalidShip
	}
	model := c.ShipByID(inst.ModelID)
	if model == nil {
		return ErrInvalidShip
	}
	grade := trackGrade(inst, t)
	cap_ := trackCap(model, t)
	if grade >= cap_ {
		return ErrMaxUpgrade
	}
	price := TrackPrice(c, model, t, grade)
	if s.Credits < price {
		return ErrInsufficientFunds
	}
	s.Credits -= price
	s.Stats.CreditsSpent += price
	setTrackGrade(inst, t, grade+1)
	return nil
}

// MaxHull returns the active ship's max hull pool: the pilot's base hull
// (content.Pilot.StartHull) plus HullPoolPerGrade per Hull track grade.
// Buying a Hull grade raises this ceiling without topping off current
// hull — ships in the hangar are otherwise kept fully maintained, but a
// Hull upgrade on the ship you're actively flying is not a free repair.
func MaxHull(s *State, c *content.Content) int {
	inst := ActiveShip(s)
	if inst == nil {
		return c.Pilot.StartHull
	}
	return c.Pilot.StartHull + c.Fleet.HullPoolPerGrade*inst.Grades.Hull
}

// HullPct returns the active ship's current hull as a 0..100 percentage of
// its max hull pool (mirrors how fuel has always been shown as a percentage
// of a variable tank size).
func HullPct(s *State, c *content.Content) float64 {
	max := MaxHull(s, c)
	if max <= 0 {
		return 0
	}
	return clamp(float64(s.Hull)/float64(max)*100, 0, 100)
}

// thrusterMul returns the active ship's Thrusters-track escape multiplier
// (< 1 is faster), before the mass penalty in slots.go is applied.
func thrusterMul(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 1
	}
	return math.Pow(c.Fleet.ThrusterEscapeMul, float64(inst.Grades.Thrusters))
}

// fuelEffMul returns the active ship's Fuel-Efficiency-track drain
// multiplier (< 1 burns less), before the mass penalty in slots.go.
func fuelEffMul(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 1
	}
	return math.Pow(c.Fleet.FuelEffMul, float64(inst.Grades.FuelEff))
}

// ScannerLockKm returns the farthest distance the active ship's Scanner can
// target: min(scanner_lock_max_km, base + per_grade*grade).
func ScannerLockKm(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	grade := 0
	if inst != nil {
		grade = inst.Grades.Scanner
	}
	km := c.Fleet.ScannerLockBaseKm + c.Fleet.ScannerLockPerGradeKm*float64(grade)
	if km > c.Fleet.ScannerLockMaxKm {
		km = c.Fleet.ScannerLockMaxKm
	}
	return km
}

// scannerEtaBonus returns the extra ETA-uncertainty reduction from a
// Scanner grade beyond B (grades A/S only), replacing the retired
// Surveyor track's ETA-narrowing effect.
func scannerEtaBonus(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	const bGrade = 3
	if inst.Grades.Scanner <= bGrade {
		return 0
	}
	return float64(inst.Grades.Scanner-bGrade) * c.Fleet.ScannerEtaBonusPct
}

func freshShipInstance(c *content.Content, modelID string) *ShipInstance {
	model := c.ShipByID(modelID)
	if model == nil {
		return nil
	}
	inst := &ShipInstance{
		ModelID: modelID,
		Grades: TrackGrades{
			Thrusters: model.ThrustersStart,
			Hull:      model.HullStart,
			FuelEff:   model.FuelEffStart,
			PowerGen:  model.PowerGenStart,
			Scanner:   model.ScannerStart,
		},
	}
	if model.UtilitySlots > 0 {
		inst.Utility = make([]*SlotDevice, model.UtilitySlots)
	}
	if model.WeaponSlots > 0 {
		inst.Weapon = make([]*SlotDevice, model.WeaponSlots)
	}
	return inst
}

func grantStarterShip(s *State, c *content.Content) {
	if s.Ships == nil {
		s.Ships = make(map[string]*ShipInstance)
	}
	if s.ShipsUnlocked == nil {
		s.ShipsUnlocked = make(map[string]bool)
	}
	s.Ships[content.StarterShipID] = freshShipInstance(c, content.StarterShipID)
	s.ShipsUnlocked[content.StarterShipID] = true
	s.ActiveShipID = content.StarterShipID
}

// ShipAcquirePrice returns the credit cost to acquire modelID: full price
// the first time, or the permanent buyback discount (BuybackPricePct,
// rounded to the nearest 10) for any model the pilot has ever owned before.
func ShipAcquirePrice(s *State, c *content.Content, modelID string) int {
	model := c.ShipByID(modelID)
	if model == nil {
		return 0
	}
	if s.ShipsUnlocked != nil && s.ShipsUnlocked[modelID] {
		return roundTo10(float64(model.Price) * c.Fleet.BuybackPricePct)
	}
	return model.Price
}

func roundTo10(v float64) int {
	return int(math.Round(v/10)) * 10
}

// IsBuyback reports whether modelID would currently price as a buyback
// (previously owned, not currently owned) rather than a first purchase.
func IsBuyback(s *State, modelID string) bool {
	return !OwnsShip(s, modelID) && s.ShipsUnlocked != nil && s.ShipsUnlocked[modelID]
}

// activateShip makes modelID (already present in s.Ships) the active ship,
// arriving fully fueled, repaired, and jammer-rearmed — hangar ships are
// assumed maintained while parked, so every path that changes which ship is
// active (purchase, switch, respawn) must apply this same reset, not just
// the fuel/hull half of it.
func activateShip(s *State, c *content.Content, modelID string) {
	s.ActiveShipID = modelID
	s.Fuel = FuelCapacity(s, c)
	s.Hull = MaxHull(s, c)
	RearmJammer(s, c)
	RearmEMPLaunchers(s)
	restoreShipShieldFull(s, c, modelID)
}

// AcquireShip buys (or buys back) modelID into the hangar and makes it the
// active ship. Requires docked, not already owned, affordable.
func AcquireShip(s *State, c *content.Content, modelID string) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if OwnsShip(s, modelID) {
		return ErrAlreadyOwned
	}
	model := c.ShipByID(modelID)
	if model == nil {
		return ErrInvalidShip
	}
	price := ShipAcquirePrice(s, c, modelID)
	if s.Credits < price {
		return ErrInsufficientFunds
	}
	s.Credits -= price
	s.Stats.CreditsSpent += price
	s.Stats.ShipsPurchased++
	if s.Ships == nil {
		s.Ships = make(map[string]*ShipInstance)
	}
	if s.ShipsUnlocked == nil {
		s.ShipsUnlocked = make(map[string]bool)
	}
	s.Ships[modelID] = freshShipInstance(c, modelID)
	s.ShipsUnlocked[modelID] = true
	activateShip(s, c, modelID)
	return nil
}

// SwitchActiveShip makes an already-owned ship active. Docked-only, free,
// instant.
func SwitchActiveShip(s *State, c *content.Content, modelID string) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if !OwnsShip(s, modelID) {
		return ErrNotOwned
	}
	if modelID == s.ActiveShipID {
		return nil
	}
	activateShip(s, c, modelID)
	return nil
}

// respawnActiveShip resolves ship loss: the destroyed ship is removed from
// the hangar forever (its grades/slots gone). If the pilot still owns other
// ships, the highest-price one becomes active — flying a hedge you already
// owned rather than being forced back to the starter. If nothing remains
// (the destroyed ship was the free starter, which can never truly be lost
// since it's always free to reacquire), a fresh starter ship is granted
// immediately so the pilot is never left shipless.
func respawnActiveShip(s *State, c *content.Content) {
	delete(s.Ships, s.ActiveShipID)
	s.Stats.ShipsLost++
	next := bestRemainingShipID(s, c)
	if next == "" {
		grantStarterShip(s, c)
	}
	activateShip(s, c, cmp.Or(next, s.ActiveShipID))
	s.WorldIdx = -1
	s.SystemID = "sol"
	s.Belt = nil
	s.Scan = nil
}

func bestRemainingShipID(s *State, c *content.Content) string {
	if len(s.Ships) == 0 {
		return ""
	}
	ids := make([]string, 0, len(s.Ships))
	for id := range s.Ships {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		mi, mj := c.ShipByID(ids[i]), c.ShipByID(ids[j])
		pi, pj := 0, 0
		if mi != nil {
			pi = mi.Price
		}
		if mj != nil {
			pj = mj.Price
		}
		if pi != pj {
			return pi > pj
		}
		return ids[i] < ids[j]
	})
	return ids[0]
}
