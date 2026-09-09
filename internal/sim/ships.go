package sim

import (
	"cmp"
	"fmt"
	"math"
	"slices"
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

// TrackDesc returns a one-line summary of what upgrading a track improves
// (docs/gameplay/05-fleet-ships-and-shipyard-economy.md "Ship upgrade
// tracks"), shown in the Shipyard when the row is selected.
func TrackDesc(t Track) string {
	switch t {
	case TrackThrusters:
		return "Faster escapes when fleeing an asteroid."
	case TrackHull:
		return "Raises max hull — more damage before the ship is lost."
	case TrackFuelEff:
		return "Burns less fuel per run."
	case TrackPowerGen:
		return "More power capacity for installed slot devices."
	case TrackScanner:
		return "Longer scan-lock range and faster scans."
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
	if inst.SystemID != s.SystemID {
		return ErrShipRemote
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
	return MaxHullFor(s, c, s.ActiveShipID)
}

// MaxHullFor returns an owned ship's maximum hull from its own Hull grade.
func MaxHullFor(s *State, c *content.Content, shipID string) int {
	inst := s.Ships[shipID]
	if inst == nil {
		return c.Pilot.StartHull
	}
	return c.Pilot.StartHull + c.Fleet.HullPoolPerGrade*inst.Grades.Hull
}

// FuelTankCapacity returns the physical capacity of an Extra Fuel Tank.
func FuelTankCapacity(c *content.Content, d *SlotDevice) float64 {
	if d == nil || d.ItemID != ItemFuelTank {
		return 0
	}
	return c.Slots.FuelTankPerGrade * float64(d.Grade+1)
}

func fuelAmountFor(s *State, c *content.Content, shipID string) float64 {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	total := clamp(inst.BaseFuel, 0, float64(c.Pilot.StartFuel))
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemFuelTank {
			total += clamp(d.Fuel, 0, FuelTankCapacity(c, d))
		}
	}
	return total
}

// FuelAmount returns fuel physically aboard the active ship: detachable
// tanks first, followed by the hull's base reserve.
func FuelAmount(s *State, c *content.Content) float64 {
	return fuelAmountFor(s, c, s.ActiveShipID)
}

// ShipFuelAmount returns fuel physically carried by a named owned ship.
func ShipFuelAmount(s *State, c *content.Content, shipID string) float64 {
	return fuelAmountFor(s, c, shipID)
}

// ShipFuelCapacity returns the total base-plus-tank capacity of a named ship.
func ShipFuelCapacity(s *State, c *content.Content, shipID string) float64 {
	return fuelCapacityFor(s, c, shipID)
}

// ShipHull returns the named hull's current physical condition.
func ShipHull(s *State, shipID string) int {
	if inst := s.Ships[shipID]; inst != nil {
		return inst.Hull
	}
	return 0
}

func fuelCapacityFor(s *State, c *content.Content, shipID string) float64 {
	inst := s.Ships[shipID]
	if inst == nil {
		return float64(c.Pilot.StartFuel)
	}
	capacity := float64(c.Pilot.StartFuel)
	for _, d := range inst.Utility {
		capacity += FuelTankCapacity(c, d)
	}
	return capacity
}

// setFuelAmountFor fills detachable tanks first, preserving the hull's base
// reserve until every installed tank is full. It is used only for migrations
// and explicit dev/legacy assignments; normal burn/refuel operations preserve
// each tank's own condition through ConsumeFuel/AddFuel.
func setFuelAmountFor(s *State, c *content.Content, shipID string, amount float64) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	remaining := clamp(amount, 0, fuelCapacityFor(s, c, shipID))
	for _, d := range inst.Utility {
		if d == nil || d.ItemID != ItemFuelTank {
			continue
		}
		cap := FuelTankCapacity(c, d)
		d.Fuel = math.Min(cap, remaining)
		remaining -= d.Fuel
	}
	inst.BaseFuel = clamp(remaining, 0, float64(c.Pilot.StartFuel))
}

// syncActiveConditionFromLegacy imports direct assignments to the retained
// State mirrors (used by deterministic tests/dev controls), then restores the
// mirrors from the now-authoritative active ShipInstance.
func syncActiveConditionFromLegacy(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	if current := FuelAmount(s, c); math.Abs(s.Fuel-current) > 0.000001 {
		setFuelAmountFor(s, c, s.ActiveShipID, s.Fuel)
	}
	if s.Hull != inst.Hull {
		inst.Hull = clampInt(s.Hull, 0, MaxHull(s, c))
	}
	syncActiveConditionMirror(s, c)
}

// syncActiveConditionMirror copies the authoritative active hull condition
// into the legacy cache before rendering or persistence.
func syncActiveConditionMirror(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	inst.BaseFuel = clamp(inst.BaseFuel, 0, float64(c.Pilot.StartFuel))
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemFuelTank {
			d.Fuel = clamp(d.Fuel, 0, FuelTankCapacity(c, d))
		}
	}
	inst.Hull = clampInt(inst.Hull, 0, MaxHull(s, c))
	s.Fuel = FuelAmount(s, c)
	s.Hull = inst.Hull
}

// ConsumeFuel drains installed tanks in utility-slot order before the active
// hull's base reserve. It returns the amount actually consumed.
func ConsumeFuel(s *State, c *content.Content, amount float64) float64 {
	if amount <= 0 || math.IsNaN(amount) {
		return 0
	}
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	remaining := amount
	for _, d := range inst.Utility {
		if d == nil || d.ItemID != ItemFuelTank || remaining <= 0 {
			continue
		}
		used := math.Min(d.Fuel, remaining)
		d.Fuel -= used
		remaining -= used
	}
	used := math.Min(inst.BaseFuel, remaining)
	inst.BaseFuel -= used
	remaining -= used
	syncActiveConditionMirror(s, c)
	return amount - remaining
}

// AddFuel fills installed tanks before the hull's base reserve and returns
// the accepted amount. Stored tanks are unaffected because they are not part
// of the active loadout.
func AddFuel(s *State, c *content.Content, amount float64) float64 {
	if amount <= 0 || math.IsNaN(amount) {
		return 0
	}
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	remaining := amount
	for _, d := range inst.Utility {
		if d == nil || d.ItemID != ItemFuelTank || remaining <= 0 {
			continue
		}
		space := FuelTankCapacity(c, d) - d.Fuel
		added := math.Min(math.Max(0, space), remaining)
		d.Fuel += added
		remaining -= added
	}
	space := float64(c.Pilot.StartFuel) - inst.BaseFuel
	added := math.Min(math.Max(0, space), remaining)
	inst.BaseFuel += added
	remaining -= added
	syncActiveConditionMirror(s, c)
	return amount - remaining
}

func setActiveHull(s *State, c *content.Content, hull int) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	inst.Hull = clampInt(hull, 0, MaxHull(s, c))
	syncActiveConditionMirror(s, c)
}

// HullPct returns the active ship's current hull as a 0..100 percentage of
// its max hull pool (mirrors how fuel has always been shown as a percentage
// of a variable tank size).
func HullPct(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	max := MaxHull(s, c)
	if max <= 0 {
		return 0
	}
	return clamp(float64(inst.Hull)/float64(max)*100, 0, 100)
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

// ScannerScanSeconds returns the active ship's sensor scan duration at a
// distance. Every Scanner grade compounds the configured speed multiplier;
// an S-grade scanner resolves contacts inside the configured instant range
// immediately (the flat scan fuel cost still applies).
func ScannerScanSeconds(s *State, c *content.Content, distanceKm float64) float64 {
	if distanceKm <= 0 {
		return 0
	}
	grade := 0
	if inst := ActiveShip(s); inst != nil {
		grade = clampInt(inst.Grades.Scanner, 0, MaxGrade)
	}
	if grade == MaxGrade && distanceKm < c.Fleet.ScannerInstantScanKm {
		return 0
	}
	return distanceKm * c.Belt.ScanSecPerKm * math.Pow(c.Fleet.ScannerScanMul, float64(grade))
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
		ModelID:            modelID,
		AdditionalInternal: make([]*SlotDevice, model.InternalSlots-1),
		Grades: TrackGrades{
			Thrusters: model.ThrustersStart,
			Hull:      model.HullStart,
			FuelEff:   model.FuelEffStart,
			PowerGen:  model.PowerGenStart,
			Scanner:   model.ScannerStart,
		},
	}
	inst.Hull = c.Pilot.StartHull + c.Fleet.HullPoolPerGrade*inst.Grades.Hull
	inst.BaseFuel = float64(c.Pilot.StartFuel)
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
	if old := s.Ships[content.StarterShipID]; old != nil && old.SystemID != s.SystemID {
		id := "skiff@" + old.SystemID
		for i := 2; s.Ships[id] != nil; i++ {
			id = fmt.Sprintf("skiff@%s-%d", old.SystemID, i)
		}
		s.Ships[id] = old
	}
	s.Ships[content.StarterShipID] = freshShipInstance(c, content.StarterShipID)
	s.Ships[content.StarterShipID].SystemID = s.SystemID
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
	if !model.NoBuyback && s.ShipsUnlocked != nil && s.ShipsUnlocked[modelID] {
		return roundTo10(float64(model.Price) * c.Fleet.BuybackPricePct)
	}
	return model.Price
}

func roundTo10(v float64) int {
	return int(math.Round(v/10)) * 10
}

// IsBuyback reports whether modelID would currently price as a buyback
// (previously owned, not currently owned) rather than a first purchase.
func IsBuyback(s *State, c *content.Content, modelID string) bool {
	model := c.ShipByID(modelID)
	return model != nil && !model.NoBuyback && ShipSoldHere(s, c, modelID) && !OwnsShip(s, modelID) && s.ShipsUnlocked != nil && s.ShipsUnlocked[modelID]
}

// activateShip makes modelID (already present in s.Ships) active without any
// service. Switching ships is a crew transfer, not a refuel/repair exploit.
func activateShip(s *State, c *content.Content, modelID string) {
	syncActiveConditionFromLegacy(s, c)
	s.ActiveShipID = modelID
	syncActiveConditionMirror(s, c)
}

// AcquireShip buys (or buys back) a fully serviced model into the hangar.
// It deliberately does not activate the ship; the pilot explicitly chooses
// whether to transfer into it afterwards.
func AcquireShip(s *State, c *content.Content, modelID string) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if modelID == content.StarterShipID && localShipCount(s) == 0 {
		grantStarterShip(s, c)
		syncActiveConditionMirror(s, c)
		return nil
	}
	if OwnsShip(s, modelID) {
		return ErrAlreadyOwned
	}
	model := c.ShipByID(modelID)
	if model == nil {
		return ErrInvalidShip
	}
	if !ShipSoldHere(s, c, modelID) {
		return ErrNotEligible
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
	s.Ships[modelID].SystemID = s.SystemID
	s.ShipsUnlocked[modelID] = true
	if price > 0 {
		s.Settings.InsuranceUsed = false
	}
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
	if s.Ships[modelID].SystemID != s.SystemID {
		return ErrShipRemote
	}
	if s.CargoUnits > CargoCapacityUnitsFor(s, c, modelID) {
		return ErrCargoDoesNotFit
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
	if s.ShipsLostAt == nil {
		s.ShipsLostAt = map[string]string{}
	}
	s.ShipsLostAt[s.ActiveShipID] = s.SystemID
	delete(s.Ships, s.ActiveShipID)
	s.Stats.ShipsLost++
	next := recoveryShipID(s, c)
	if next == "" {
		grantStarterShip(s, c)
		// grantStarterShip made a fresh active hull; hydrate the legacy cache
		// before activateShip's compatibility import can see stale zero hull.
		syncActiveConditionMirror(s, c)
	}
	activateShip(s, c, cmp.Or(next, s.ActiveShipID))
	s.WorldIdx = -1
	s.Belt = nil
	s.Scan = nil
}

// recoveryShipID chooses the documented recovery default: an existing Skiff
// if one survives, otherwise the best remaining hull. It never services the
// chosen backup as part of selection.
func recoveryShipID(s *State, c *content.Content) string {
	if OwnsShip(s, content.StarterShipID) && s.Ships[content.StarterShipID].SystemID == s.SystemID {
		return content.StarterShipID
	}
	return bestRemainingShipID(s, c)
}

func bestRemainingShipID(s *State, c *content.Content) string {
	if len(s.Ships) == 0 {
		return ""
	}
	ids := make([]string, 0, len(s.Ships))
	for id, inst := range s.Ships {
		if inst.SystemID == s.SystemID {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ""
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

func localShipCount(s *State) int {
	n := 0
	for _, ship := range s.Ships {
		if ship.SystemID == s.SystemID {
			n++
		}
	}
	return n
}
func ShipSoldHere(s *State, c *content.Content, id string) bool {
	model := c.ShipByID(id)
	if model == nil {
		return false
	}
	return id == content.StarterShipID || slices.Contains(model.SoldIn, s.SystemID) || !model.NoBuyback && s.ShipsUnlocked[id] && s.ShipsLostAt[id] == s.SystemID
}
func localMinTravelFuel(s *State, c *content.Content) int {
	n := int(^uint(0) >> 1)
	for i, w := range c.Worlds {
		if w.SystemID == s.SystemID && RouteLockReason(s, c, i) == "" {
			n = min(n, w.TravelFuel)
		}
	}
	return n
}

// HangarIDs includes catalog hulls and locally issued rescue Skiffs without
// hiding or relocating a Skiff already parked elsewhere.
func HangarIDs(s *State, c *content.Content) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, m := range c.Ships {
		ids = append(ids, m.ID)
		seen[m.ID] = true
	}
	extra := []string{}
	for id := range s.Ships {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	return append(ids, extra...)
}
