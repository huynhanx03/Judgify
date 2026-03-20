package geo

import (
	"fmt"
	"math"

	"github.com/huynhanx03/judgify/pkg/utils"
)

// Limits from EPSG:900913 / EPSG:3785 / OSGEO:41001
const (
	LatMin              float64 = -85.05112878
	LatMax              float64 = 85.05112878
	LongMin             float64 = -180
	LongMax             float64 = 180
	DR                  float64 = math.Pi / 180.0
	EarthRadiusInMeters float64 = 6372797.560856
	MercatorMax         float64 = 20037726.37

	// 52-bits gives us accuracy down to 0.6m
	MaxStep uint8 = 26
)

// CoordRange represents the valid range for coordinates.
type Range struct {
	MinLat  float64
	MaxLat  float64
	MinLong float64
	MaxLong float64
}

// Standard ranges
var (
	CoordRange = Range{
		MinLat:  LatMin,
		MaxLat:  LatMax,
		MinLong: LongMin,
		MaxLong: LongMax,
	}

	StandardRange = Range{
		MinLat:  -90,
		MaxLat:  90,
		MinLong: -180,
		MaxLong: 180,
	}

	// neighborDX and neighborDY define the 8 direction offsets: N, S, E, W, NE, NW, SE, SW.
	neighborDX = [8]int{-1, 1, 0, 0, -1, -1, 1, 1}
	neighborDY = [8]int{0, 0, 1, -1, 1, -1, 1, -1}
)

// Bits represents an encoded Geohash.
type Bits struct {
	Step uint8
	Bits uint64
}

// Point represents a geographical location.
type Point struct {
	Longitude float64
	Latitude  float64
}

// Area represents a decoded Geohash bounding box.
type Area struct {
	Hash   Bits
	GRange Range
}

// Encode converts Longitude and Latitude to a Geohash bit representation.
func Encode(geoRange Range, long float64, lat float64, step uint8) (*Bits, error) {
	if long > geoRange.MaxLong || long < geoRange.MinLong ||
		lat > geoRange.MaxLat || lat < geoRange.MinLat {
		return nil, fmt.Errorf("invalid coord: lon %f, lat %f", long, lat)
	}

	res := &Bits{
		Step: step,
		Bits: 0,
	}

	latOffset := (lat - geoRange.MinLat) / (geoRange.MaxLat - geoRange.MinLat)
	longOffset := (long - geoRange.MinLong) / (geoRange.MaxLong - geoRange.MinLong)
	exp2Step := 1 << step
	latOffset *= float64(exp2Step)
	longOffset *= float64(exp2Step)

	// lat is at even position, long is at odd position
	res.Bits = interleave(uint32(latOffset), uint32(longOffset))
	return res, nil
}

// Decode converts a Geohash bit representation back to an Area bounding box.
func Decode(geoRange Range, hash Bits) Area {
	var step = hash.Step
	latBits, longBits := deinterleave(hash.Bits)
	latScale := geoRange.MaxLat - geoRange.MinLat
	longScale := geoRange.MaxLong - geoRange.MinLong
	exp2Step := 1 << step

	return Area{
		Hash: hash,
		GRange: Range{
			MinLat:  geoRange.MinLat + (float64(latBits)/float64(exp2Step))*latScale,
			MaxLat:  geoRange.MinLat + (float64(latBits+1)/float64(exp2Step))*latScale,
			MinLong: geoRange.MinLong + (float64(longBits)/float64(exp2Step))*longScale,
			MaxLong: geoRange.MinLong + (float64(longBits+1)/float64(exp2Step))*longScale,
		},
	}
}

// DecodeToLongLat decodes a Geohash and returns the center Longitude and Latitude.
func DecodeToLongLat(geoRange Range, hash Bits) (long float64, lat float64) {
	area := Decode(geoRange, hash)
	// result is the center of the rectangle
	lat = (area.GRange.MinLat + area.GRange.MaxLat) / 2
	long = (area.GRange.MinLong + area.GRange.MaxLong) / 2

	if lat > LatMax {
		lat = LatMax
	} else if lat < LatMin {
		lat = LatMin
	}

	if long > LongMax {
		long = LongMax
	} else if long < LongMin {
		long = LongMin
	}
	return long, lat
}

// Distance calculates distance using haversine great circle distance formula. Returns distance in meters.
func Distance(lon1 float64, lat1 float64, lon2 float64, lat2 float64) float64 {
	lon1r := degToRad(lon1)
	lon2r := degToRad(lon2)
	v := math.Sin((lon2r - lon1r) / 2.0)
	if v == 0.0 {
		return getLatDistance(lat1, lat2)
	}
	lat1r := degToRad(lat1)
	lat2r := degToRad(lat2)
	u := math.Sin((lat2r - lat1r) / 2.0)
	a := u*u + math.Cos(lat1r)*math.Cos(lat2r)*v*v
	return 2.0 * EarthRadiusInMeters * math.Asin(math.Sqrt(a))
}

// Align52Bits aligns a hash to 52 bits for standardized storage as a score in a zset.
func Align52Bits(hash Bits) uint64 {
	return hash.Bits << (52 - hash.Step*2)
}

// Neighbors returns the 8 Geohash cells surrounding the given hash at the same step.
func (h Bits) Neighbors() []Bits {
	latBits, longBits := deinterleave(h.Bits)
	res := make([]Bits, 0, 8)

	maxVal := uint32(1<<h.Step) - 1

	for i := 0; i < 8; i++ {
		nx := int(longBits) + neighborDX[i]
		ny := int(latBits) + neighborDY[i]

		// Check boundaries (Geohash usually wraps longitude but not latitude,
		// but simple implementation can just skip invalid lat)
		if ny < 0 || ny > int(maxVal) {
			continue
		}

		// Wrap long
		if nx < 0 {
			nx = int(maxVal)
		} else if nx > int(maxVal) {
			nx = 0
		}

		res = append(res, Bits{
			Step: h.Step,
			Bits: interleave(uint32(ny), uint32(nx)),
		})
	}

	return res
}

// GetSearchRanges returns the min/max 52-bit scores for the 9 cells (self + neighbors)
// that cover the search area.
func GetSearchRanges(geoRange Range, long float64, lat float64, radiusMeters float64) [][2]uint64 {
	step := EstimateStepByRadius(radiusMeters)
	if step > 26 {
		step = 26
	}

	center, err := Encode(geoRange, long, lat, step)
	if err != nil {
		return nil
	}

	cells := []Bits{*center}
	cells = append(cells, center.Neighbors()...)

	ranges := make([][2]uint64, 0, len(cells))
	for _, cell := range cells {
		// A cell's range is [Align52Bits(cell), Align52Bits(cell+1)] effectively
		// But Geohash is hierarchical. bits << 52-2*step is the min score.
		// min + (1 << 52-2*step) is the next cell's start.
		min := Align52Bits(cell)
		max := min + (1 << (52 - 2*cell.Step)) - 1
		ranges = append(ranges, [2]uint64{min, max})
	}
	return ranges
}

// EstimateStepByRadius estimates the Geohash step (precision length) required to cover a given radius.
func EstimateStepByRadius(radiusMeters float64) uint8 {
	var step uint8 = 1
	for radiusMeters < MercatorMax {
		radiusMeters *= 2
		step++
	}
	step -= 2
	return step
}

// --- Internal Math Helpers ---

func degToRad(angle float64) float64 {
	return angle * DR
}

func getLatDistance(lat1 float64, lat2 float64) float64 {
	return EarthRadiusInMeters * math.Abs(degToRad(lat2)-degToRad(lat1))
}

func interleave(x uint32, y uint32) uint64 {
	return utils.Spread32(x) | (utils.Spread32(y) << 1)
}

func deinterleave(x uint64) (uint32, uint32) {
	return utils.Squash64(x), utils.Squash64(x >> 1)
}
